package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

var _ = Describe("HermesSelfConfig controller", func() {
	const (
		ns = "default"
		// Idempotency-check timeout: 30s was too short on stock envtest,
		// 60s still flaked on k8s 1.32 (parent HermesInstance reconciler
		// races with the SelfConfig reconciler on slow runners). 120s
		// absorbs the worst-case settle time.
		timeout = 120 * time.Second
		poll    = 200 * time.Millisecond
	)

	AfterEach(func() {
		ctx := context.Background()
		for _, name := range []string{"deny-target", "happy-target", "patch-target"} {
			_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}})
			// envtest runs no garbage collector: drop the rendered config so
			// the next test starts from a fresh ConfigMap.
			_ = k8sClient.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: ns}})
		}
		scs := &hermesv1.HermesSelfConfigList{}
		_ = k8sClient.List(ctx, scs, &client.ListOptions{Namespace: ns})
		for i := range scs.Items {
			_ = k8sClient.Delete(ctx, &scs.Items[i])
		}
	})

	It("denies a SelfConfig whose parent has selfConfigure.enabled=false", func() {
		ctx := context.Background()
		parent := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-target", Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/paperclipinc/hermes-agent",
					Tag:        "test",
				},
				// SelfConfigure.Enabled left nil/false on purpose.
			},
		}
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())

		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-this", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: "deny-target",
				AddSkills:   []hermesv1.SelfConfigSkill{{Source: "git+x"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "deny-this", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseDenied))
			g.Expect(got.Status.DenyReason).To(ContainSubstring("selfconfig disabled"))
		}).Within(timeout).WithPolling(poll).Should(Succeed())
	})

	It("is idempotent: re-reconciling the same generation does not bump observedGeneration twice", func() {
		ctx := context.Background()
		trueP := true

		parent := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "happy-target", Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/paperclipinc/hermes-agent",
					Tag:        "test",
				},
				SelfConfigure: hermesv1.SelfConfigureSpec{
					Enabled:        &trueP,
					AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionEnvVars},
					ProtectedKeys:  []string{"provider.*"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())

		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "idem-test", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: "happy-target",
				AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		first := &hermesv1.HermesSelfConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, first)).To(Succeed())
		firstApplied := first.Status.AppliedAt

		// Poke an unrelated annotation on the SelfConfig to force a re-reconcile.
		Eventually(func() error {
			var cur hermesv1.HermesSelfConfig
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, &cur); err != nil {
				return err
			}
			if cur.Annotations == nil {
				cur.Annotations = map[string]string{}
			}
			cur.Annotations["test.example.com/poke"] = time.Now().String()
			return k8sClient.Update(ctx, &cur)
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		time.Sleep(2 * time.Second)

		second := &hermesv1.HermesSelfConfig{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "idem-test", Namespace: ns}, second)).To(Succeed())
		Expect(second.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		Expect(second.Status.AppliedAt.Equal(firstApplied)).To(BeTrue(),
			"AppliedAt must not advance on no-op reconciles: the controller short-circuits via ObservedGeneration")
	})

	// createPatchTarget creates a selfConfigure-enabled parent plus a
	// HermesSelfConfig carrying patchConfig, and waits for the patch to land
	// in the rendered config ConfigMap.
	createPatchTarget := func(ctx context.Context) {
		trueP := true
		parent := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "patch-target", Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/paperclipinc/hermes-agent",
					Tag:        "test",
				},
				Config: hermesv1.ConfigSpec{
					Raw: &hermesv1.RawConfig{RawExtension: runtime.RawExtension{
						Raw: []byte(`{"model":"gpt-4o","memory":{"provider":"builtin"}}`),
					}},
				},
				SelfConfigure: hermesv1.SelfConfigureSpec{
					Enabled:        &trueP,
					AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionConfig},
				},
			},
		}
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())

		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "patch-memory", Namespace: ns},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: "patch-target",
				PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{"memory":{"provider":"hindsight"}}`)},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "patch-memory", Namespace: ns}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
			g.Expect(got.Status.AppliedFields).To(ContainElement(AppliedFieldPatchConfig))
		}).Within(timeout).WithPolling(poll).Should(Succeed())
	}

	patchedConfigBody := func(ctx context.Context) string {
		cm := &corev1.ConfigMap{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "patch-target-config", Namespace: ns}, cm); err != nil {
			return ""
		}
		return cm.Data["config.yaml"]
	}

	It("patchConfig ends up in the rendered config.yaml", func() {
		ctx := context.Background()
		createPatchTarget(ctx)

		Eventually(func(g Gomega) {
			body := patchedConfigBody(ctx)
			g.Expect(body).To(ContainSubstring("provider: hindsight"))
			g.Expect(body).To(ContainSubstring("model: gpt-4o"), "user config must survive the patch")
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		// The old design wrote selfconfig.yaml into the workspace ConfigMap,
		// which nothing consumed. That key must no longer appear.
		ws := &corev1.ConfigMap{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "patch-target-workspace", Namespace: ns}, ws); err == nil {
			Expect(ws.Data).NotTo(HaveKey("selfconfig.yaml"))
		}
	})

	It("patchConfig survives an instance reconcile and is removed with its HermesSelfConfig", func() {
		ctx := context.Background()
		createPatchTarget(ctx)
		Eventually(func() string { return patchedConfigBody(ctx) }).
			Within(timeout).WithPolling(poll).Should(ContainSubstring("provider: hindsight"))

		// Touch a label on the parent to force a full instance reconcile.
		Eventually(func() error {
			var cur hermesv1.HermesInstance
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: "patch-target", Namespace: ns}, &cur); err != nil {
				return err
			}
			if cur.Labels == nil {
				cur.Labels = map[string]string{}
			}
			cur.Labels["test.example.com/poke"] = "1"
			return k8sClient.Update(ctx, &cur)
		}).Within(timeout).WithPolling(poll).Should(Succeed())

		Consistently(func() string { return patchedConfigBody(ctx) }).
			Within(5*time.Second).WithPolling(poll).Should(ContainSubstring("provider: hindsight"),
			"instance reconcile must not wipe an admitted patchConfig")

		// Deleting the request retracts the patch.
		Expect(k8sClient.Delete(ctx, &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "patch-memory", Namespace: ns},
		})).To(Succeed())
		Eventually(func() string { return patchedConfigBody(ctx) }).
			Within(timeout).WithPolling(poll).Should(SatisfyAll(
			ContainSubstring("provider: builtin"),
			Not(ContainSubstring("hindsight")),
		))
	})
})
