/*
Copyright 2026 stubbi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// Ptr returns a pointer to v. Local test helper: mirrors resources.Ptr but
// avoids importing the internal package from the test file.
func Ptr[T any](v T) *T { return &v }

var _ = Describe("HermesInstance controller", func() {
	const (
		name      = "demo"
		namespace = "default"
		// 30s is enough on most k8s envtest versions; bumped to 60s because
		// k8s 1.32 envtest can lag on AfterEach cleanup (reconciler can race
		// with the test's explicit STS delete).
		timeout  = 60 * time.Second
		interval = 250 * time.Millisecond
	)

	AfterEach(func() {
		ctx := context.Background()
		// Delete the HermesInstance first and wait for it to disappear so the
		// controller stops reconciling. Then delete owned resources explicitly
		// (envtest does not run the k8s garbage collector).
		inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		_ = k8sClient.Delete(ctx, inst)
		Eventually(func() error {
			obj := &hermesv1.HermesInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("HermesInstance %s still exists", name)
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
		_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
		_ = k8sClient.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace}})
		_ = k8sClient.Delete(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name + "-data", Namespace: namespace}})

		// Wait for the StatefulSet to be gone before the next test starts.
		Eventually(func() error {
			sts := &appsv1.StatefulSet{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("StatefulSet %s still exists", name)
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("creates PVC, ConfigMap, Service, and StatefulSet for a new HermesInstance", func() {
		ctx := context.Background()

		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "test",
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			pvc := &corev1.PersistentVolumeClaim{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-data", Namespace: namespace}, pvc)).To(Succeed())
			cm := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-config", Namespace: namespace}, cm)).To(Succeed())
			svc := &corev1.Service{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, svc)).To(Succeed())
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			g.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/stubbi/hermes-agent:test"))
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("is idempotent: second reconcile does not change StatefulSet generation", func() {
		ctx := context.Background()

		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		var stsGenBefore int64
		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			stsGenBefore = sts.Generation
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		Eventually(func() error {
			var cur hermesv1.HermesInstance
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur); err != nil {
				return err
			}
			if cur.Annotations == nil {
				cur.Annotations = map[string]string{}
			}
			cur.Annotations["test.example.com/poke"] = time.Now().String()
			return k8sClient.Update(ctx, &cur)
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		time.Sleep(2 * time.Second)

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
		Expect(sts.Generation).To(Equal(stsGenBefore), "STS generation must not bump on no-op reconcile")
	})
})

var _ = Describe("HermesInstance: full subsystems", func() {
	const (
		name      = "demo-full"
		namespace = "default"
		timeout   = 60 * time.Second
		interval  = 250 * time.Millisecond
	)

	AfterEach(func() {
		inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		_ = k8sClient.Delete(context.Background(), inst)
	})

	It("creates per-subsystem resources for a maximal HermesInstance", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			np := &networkingv1.NetworkPolicy{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())
			sa := &corev1.ServiceAccount{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sa)).To(Succeed())
			role := &rbacv1.Role{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, role)).To(Succeed())
			rb := &rbacv1.RoleBinding{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, rb)).To(Succeed())
			pdb := &policyv1.PodDisruptionBudget{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, pdb)).To(Succeed())
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, hpa)).To(Succeed())
			ing := &networkingv1.Ingress{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)).To(Succeed())
			sec := &corev1.Secret{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-gateway-tokens", Namespace: namespace}, sec)).To(Succeed())
			ws := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-workspace", Namespace: namespace}, ws)).To(Succeed())
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("is idempotent across the FULL spec (10 reconciles, no STS generation bump)", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		var stsGen int64
		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			g.Expect(sts.Generation).To(BeNumerically(">=", int64(1)))
			stsGen = sts.Generation
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		for i := 0; i < 10; i++ {
			var cur hermesv1.HermesInstance
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur)).To(Succeed())
			if cur.Annotations == nil {
				cur.Annotations = map[string]string{}
			}
			cur.Annotations["test.example.com/poke"] = fmt.Sprintf("%d-%d", i, time.Now().UnixNano())
			Expect(k8sClient.Update(ctx, &cur)).To(Succeed())
			time.Sleep(500 * time.Millisecond)
		}

		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
		Expect(sts.Generation).To(Equal(stsGen), "STS generation must not bump on no-op reconciles")
	})

	It("scales to zero replicas when spec.suspended=true", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		inst.Spec.Suspended = true
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			sts := &appsv1.StatefulSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)).To(Succeed())
			g.Expect(sts.Spec.Replicas).ToNot(BeNil())
			g.Expect(*sts.Spec.Replicas).To(Equal(int32(0)))
		}).Within(timeout).WithPolling(interval).Should(Succeed())
	})

	It("deletes the Ingress when spec.networking.ingress.enabled is flipped to false", func() {
		ctx := context.Background()
		inst := maximalInstance(name, namespace)
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			ing := &networkingv1.Ingress{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)).To(Succeed())
		}).Within(timeout).WithPolling(interval).Should(Succeed())

		var cur hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &cur)).To(Succeed())
		cur.Spec.Networking.Ingress.Enabled = Ptr(false)
		Expect(k8sClient.Update(ctx, &cur)).To(Succeed())

		Eventually(func() bool {
			ing := &networkingv1.Ingress{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ing)
			return apierrors.IsNotFound(err)
		}).Within(timeout).WithPolling(interval).Should(BeTrue())
	})
})

var _ = Describe("HermesInstance reconciler: gateways", func() {
	const (
		instName = "gateways-it"
		ns       = "default"
	)

	BeforeEach(func() {
		ctx := context.Background()
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tg-secret", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("xxxx")},
		}
		_ = k8sClient.Create(ctx, sec)
	})

	AfterEach(func() {
		ctx := context.Background()
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tg-secret", Namespace: ns}})
	})

	It("propagates gateway env vars into the StatefulSet and an egress rule into the NetworkPolicy", func() {
		ctx := context.Background()
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
				Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled: Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"},
							Key:                  "token",
						},
						AllowedUserIDs: []int64{42},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts)).To(Succeed())
			env := sts.Spec.Template.Spec.Containers[0].Env
			byName := map[string]corev1.EnvVar{}
			for _, e := range env {
				byName[e.Name] = e
			}
			g.Expect(byName).To(HaveKey("TELEGRAM_BOT_TOKEN"))
			g.Expect(byName["TELEGRAM_BOT_TOKEN"].ValueFrom).NotTo(BeNil())
			g.Expect(byName["TELEGRAM_ALLOWED_USER_IDS"].Value).To(Equal("42"))
		}, "30s", "250ms").Should(Succeed())

		Eventually(func(g Gomega) {
			var np networkingv1.NetworkPolicy
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &np)).To(Succeed())
			has443 := false
			for _, r := range np.Spec.Egress {
				for _, p := range r.Ports {
					if p.Port != nil && p.Port.IntVal == 443 {
						has443 = true
					}
				}
			}
			g.Expect(has443).To(BeTrue(), "egress rule for gateway endpoints (443/TCP)")
		}, "30s", "250ms").Should(Succeed())
	})

	It("removes gateway env vars when the gateway is toggled off", func() {
		ctx := context.Background()
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
				Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled:           Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "tg-secret"}, Key: "token"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func() bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts); err != nil {
				return false
			}
			for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "TELEGRAM_BOT_TOKEN" {
					return true
				}
			}
			return false
		}, "30s", "250ms").Should(BeTrue())

		var live hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
		live.Spec.Gateways.Telegram.Enabled = Ptr(false)
		Expect(k8sClient.Update(ctx, &live)).To(Succeed())

		Eventually(func() bool {
			var sts appsv1.StatefulSet
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts); err != nil {
				return true
			}
			for _, e := range sts.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "TELEGRAM_BOT_TOKEN" {
					return false
				}
			}
			return true
		}, "30s", "250ms").Should(BeTrue(), "TELEGRAM_BOT_TOKEN env var removed after toggle")
	})
})

var _ = Describe("HermesInstance reconciler: Honcho profile store", func() {
	const (
		instName = "honcho-it"
		ns       = "default"
	)

	AfterEach(func() {
		ctx := context.Background()
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "honcho-secret", Namespace: ns}})
	})

	It("creates Honcho Deployment+Service+PVC when honcho.enabled=true", func() {
		ctx := context.Background()
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho-secret", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
				Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled:         Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"}, Key: "api-key"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		Eventually(func(g Gomega) {
			var dep appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep)).To(Succeed())
			var svc corev1.Service
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &svc)).To(Succeed())
			var pvc corev1.PersistentVolumeClaim
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho-data", Namespace: ns}, &pvc)).To(Succeed())
		}, "30s", "250ms").Should(Succeed())

		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &sts)).To(Succeed())
			env := sts.Spec.Template.Spec.Containers[0].Env
			byName := map[string]corev1.EnvVar{}
			for _, e := range env {
				byName[e.Name] = e
			}
			g.Expect(byName).To(HaveKey("HONCHO_BASE_URL"))
			g.Expect(byName["HONCHO_BASE_URL"].Value).To(Equal("http://" + instName + "-honcho:8000"))
		}, "30s", "250ms").Should(Succeed())
	})

	It("deletes Honcho Deployment+Service when toggled off (PVC retained)", func() {
		ctx := context.Background()
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho-secret", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
				Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled:         Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho-secret"}, Key: "api-key"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())
		Eventually(func() error {
			var dep appsv1.Deployment
			return k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep)
		}, "30s", "250ms").Should(Succeed())

		var live hermesv1.HermesInstance
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
		live.Spec.ProfileStore.Honcho.Enabled = Ptr(false)
		Expect(k8sClient.Update(ctx, &live)).To(Succeed())

		Eventually(func() bool {
			var dep appsv1.Deployment
			return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &dep))
		}, "30s", "250ms").Should(BeTrue())
		Eventually(func() bool {
			var svc corev1.Service
			return apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho", Namespace: ns}, &svc))
		}, "30s", "250ms").Should(BeTrue())

		var pvc corev1.PersistentVolumeClaim
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName + "-honcho-data", Namespace: ns}, &pvc)).To(Succeed())
	})
})

var _ = Describe("HermesInstance reconciler: idempotency canary (Plan 3 surface)", func() {
	const (
		instName = "canary-plan3"
		ns       = "default"
	)

	AfterEach(func() {
		ctx := context.Background()
		_ = k8sClient.Delete(ctx, &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tg", Namespace: ns}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "honcho", Namespace: ns}})
	})

	It("does not bump StatefulSet generation after 10 reconciles with full Plan 3 spec", func() {
		ctx := context.Background()
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tg", Namespace: ns},
			Data:       map[string][]byte{"token": []byte("x")},
		})
		_ = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "honcho", Namespace: ns},
			Data:       map[string][]byte{"api-key": []byte("k")},
		})

		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instName, Namespace: ns},
			Spec: hermesv1.HermesInstanceSpec{
				Image:   hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent"},
				Storage: hermesv1.StorageSpec{Persistence: hermesv1.PersistenceSpec{Size: "1Gi"}},
				Runtime: hermesv1.RuntimeSpec{
					UV:               hermesv1.UVSpec{Enabled: Ptr(true)},
					ExtraPipPackages: []string{"polars"},
				},
				Gateways: hermesv1.GatewaysSpec{
					Telegram: hermesv1.TelegramGatewaySpec{
						Enabled:           Ptr(true),
						BotTokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "tg"}, Key: "token"},
						AllowedUserIDs:    []int64{42, 1337},
					},
				},
				ProfileStore: hermesv1.ProfileStoreSpec{
					Honcho: hermesv1.HonchoSpec{
						Enabled:         Ptr(true),
						APIKeySecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "honcho"}, Key: "api-key"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, inst)).To(Succeed())

		var stsAfterFirst appsv1.StatefulSet
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &stsAfterFirst)).To(Succeed())
			g.Expect(stsAfterFirst.Generation).To(BeNumerically(">=", int64(1)))
		}, "30s", "250ms").Should(Succeed())

		for i := 0; i < 10; i++ {
			var live hermesv1.HermesInstance
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &live)).To(Succeed())
			if live.Annotations == nil {
				live.Annotations = map[string]string{}
			}
			live.Annotations["hermes.agent/canary-tick"] = fmt.Sprintf("%d-%d", i, time.Now().UnixNano())
			Expect(k8sClient.Update(ctx, &live)).To(Succeed())
			time.Sleep(300 * time.Millisecond)
		}

		var stsAfterTen appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: instName, Namespace: ns}, &stsAfterTen)).To(Succeed())
		Expect(stsAfterTen.Generation).To(Equal(stsAfterFirst.Generation),
			"StatefulSet generation must not advance under repeat reconciles with unchanged spec")
	})
})

func maximalInstance(name, namespace string) *hermesv1.HermesInstance {
	tp := int32(8443)
	mi := intstr.FromString("50%")
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "test", PullPolicy: "IfNotPresent"},
			Storage: hermesv1.StorageSpec{
				Persistence: hermesv1.PersistenceSpec{Enabled: Ptr(true), Size: "1Gi"},
			},
			Resources: hermesv1.ResourcesSpec{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			},
			Security: hermesv1.SecuritySpec{
				NetworkPolicy: hermesv1.NetworkPolicySpec{Enabled: Ptr(true), AllowDNS: Ptr(true)},
				RBAC:          hermesv1.RBACSpec{CreateServiceAccount: Ptr(true)},
				CABundle:      hermesv1.CABundleSpec{ConfigMapName: "no-such-cm", Key: "ca.crt"},
			},
			Networking: hermesv1.NetworkingSpec{
				Service: hermesv1.ServiceSpec{
					Type:  corev1.ServiceTypeClusterIP,
					Ports: []hermesv1.NamedServicePort{{Name: "gateway", Port: 8443, TargetPort: &tp, Protocol: corev1.ProtocolTCP}},
				},
				Ingress: hermesv1.IngressSpec{
					Enabled: Ptr(true), Host: "demo.example.com", ClassName: Ptr("nginx"),
					ServicePortName: "gateway", PathType: networkingv1.PathTypePrefix, Path: "/",
				},
			},
			Observability: hermesv1.ObservabilitySpec{
				Metrics:        hermesv1.MetricsSpec{Enabled: Ptr(true), Port: 9090, Secure: Ptr(false)},
				ServiceMonitor: hermesv1.ServiceMonitorSpec{Enabled: Ptr(true)},
				PrometheusRule: hermesv1.PrometheusRuleSpec{Enabled: Ptr(true)},
				Logging:        hermesv1.LoggingSpec{Format: hermesv1.LogFormatJSON, Level: "info"},
			},
			Availability: hermesv1.AvailabilitySpec{
				PodDisruptionBudget: hermesv1.PDBSpec{Enabled: Ptr(true), MinAvailable: &mi},
				HorizontalPodAutoscaler: hermesv1.HPASpec{
					Enabled: Ptr(true), MinReplicas: Ptr(int32(1)), MaxReplicas: Ptr(int32(3)),
					TargetCPUUtilization: Ptr(int32(70)),
				},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.ScheduleAnyway, MaxSkew: 1,
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}}},
				},
			},
			Workspace: hermesv1.WorkspaceSpec{
				InitialFiles: []hermesv1.WorkspaceFile{{Path: "notes/finance.md", Content: "x"}},
				InitialDirs:  []string{"data"},
			},
			Scheduling: hermesv1.SchedulingSpec{NodeSelector: map[string]string{"disktype": "ssd"}},
			Env:        []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
		},
	}
}
