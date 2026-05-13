package controller

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("HermesSelfConfig — GitOps coexistence (SSA)", func() {
	const (
		ssaNS      = "default"
		ssaName    = "ssa-target"
		ssaTimeout = 45 * time.Second
		ssaPoll    = 250 * time.Millisecond
	)

	AfterEach(func() {
		ctx := context.Background()
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: ssaName, Namespace: ssaNS}})
		scs := &hermesv1.HermesSelfConfigList{}
		_ = k8sClient.List(ctx, scs, &client.ListOptions{Namespace: ssaNS})
		for i := range scs.Items {
			_ = k8sClient.Delete(ctx, &scs.Items[i])
		}
	})

	It("preserves Flux-owned fields while applying SelfConfig-owned fields", func() {
		ctx := context.Background()
		trueP := true

		// 1. "Flux" applies a HermesInstance with image.tag=v1.0.0.
		fluxApplied := &hermesv1.HermesInstance{
			TypeMeta:   metav1.TypeMeta{APIVersion: hermesv1.GroupVersion.String(), Kind: "HermesInstance"},
			ObjectMeta: metav1.ObjectMeta{Name: ssaName, Namespace: ssaNS},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "v1.0.0",
				},
				SelfConfigure: hermesv1.SelfConfigureSpec{
					Enabled:        &trueP,
					AllowedActions: []hermesv1.SelfConfigAction{hermesv1.ActionEnvVars},
				},
			},
		}
		Expect(k8sClient.Patch(ctx, fluxApplied, client.Apply,
			client.FieldOwner("flux-controller"),
			client.ForceOwnership,
		)).To(Succeed())

		// 2. SelfConfig adds TZ=UTC via its reconciler.
		sc := &hermesv1.HermesSelfConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "ssa-sc-1", Namespace: ssaNS},
			Spec: hermesv1.HermesSelfConfigSpec{
				InstanceRef: ssaName,
				AddEnvVars:  []hermesv1.SelfConfigEnvVar{{Name: "TZ", Value: "UTC"}},
			},
		}
		Expect(k8sClient.Create(ctx, sc)).To(Succeed())

		// 3. Wait for Applied.
		Eventually(func(g Gomega) {
			got := &hermesv1.HermesSelfConfig{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "ssa-sc-1", Namespace: ssaNS}, got)).To(Succeed())
			g.Expect(got.Status.Phase).To(Equal(hermesv1.SelfConfigPhaseApplied))
		}).Within(ssaTimeout).WithPolling(ssaPoll).Should(Succeed())

		// 4. Image.Tag unchanged (Flux still owns it); Env[TZ] present and owned by SelfConfig.
		inst := &hermesv1.HermesInstance{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ssaName, Namespace: ssaNS}, inst)).To(Succeed())
		Expect(inst.Spec.Image.Tag).To(Equal("v1.0.0"))
		Expect(inst.Spec.Env).To(ContainElement(corev1.EnvVar{Name: "TZ", Value: "UTC"}))

		fluxOwnsImage := false
		selfconfigOwnsEnv := false
		for _, mf := range inst.ManagedFields {
			if mf.FieldsV1 == nil {
				continue
			}
			fields := string(mf.FieldsV1.Raw)
			if mf.Manager == "flux-controller" && strings.Contains(fields, `"f:image"`) {
				fluxOwnsImage = true
			}
			if mf.Manager == SelfConfigFieldManager && strings.Contains(fields, `"f:env"`) {
				selfconfigOwnsEnv = true
			}
		}
		Expect(fluxOwnsImage).To(BeTrue(), "flux-controller must still own spec.image")
		Expect(selfconfigOwnsEnv).To(BeTrue(), "hermes.agent/selfconfig must own spec.env")

		// 5. Flux re-applies with a different tag — env var must survive.
		fluxApplied2 := fluxApplied.DeepCopy()
		fluxApplied2.Spec.Image.Tag = "v1.0.1"
		fluxApplied2.ResourceVersion = ""
		fluxApplied2.ManagedFields = nil
		Expect(k8sClient.Patch(ctx, fluxApplied2, client.Apply,
			client.FieldOwner("flux-controller"),
			client.ForceOwnership,
		)).To(Succeed())

		inst2 := &hermesv1.HermesInstance{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ssaName, Namespace: ssaNS}, inst2)).To(Succeed())
			g.Expect(inst2.Spec.Image.Tag).To(Equal("v1.0.1"))
			g.Expect(inst2.Spec.Env).To(ContainElement(corev1.EnvVar{Name: "TZ", Value: "UTC"}))
		}).Within(ssaTimeout).WithPolling(ssaPoll).Should(Succeed(), "env var must survive Flux re-apply — no flap")
	})
})
