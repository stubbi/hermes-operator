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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("Migration sub-controller", func() {
	const (
		mName     = "demo-migrate"
		namespace = "default"
		timeout   = 30 * time.Second
		interval  = 250 * time.Millisecond
	)

	ctx := context.Background()

	newMigrationInstanceOpenClaw := func() *hermesv1.HermesInstance {
		return &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mName,
				Namespace: namespace,
			},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "1.0.0",
				},
				Migration: hermesv1.MigrationSpec{
					FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
						Source: hermesv1.MigrationFromOpenClawSource{
							OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{
								Name:      "my-openclaw",
								Namespace: namespace,
							},
						},
						Mode: "copy",
					},
				},
			},
		}
	}

	AfterEach(func() {
		inst := &hermesv1.HermesInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, inst); err == nil {
			if len(inst.Finalizers) > 0 {
				original := inst.DeepCopy()
				inst.Finalizers = nil
				_ = k8sClient.Patch(ctx, inst, client.MergeFrom(original))
			}
			_ = k8sClient.Delete(ctx, inst)
		}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, inst)
		}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))
	})

	Context("init-migrate-from-openclaw injection", func() {
		It("injects init-migrate-from-openclaw when openclawInstanceRef mode is set", func() {
			inst := newMigrationInstanceOpenClaw()
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// The controller should create a STS with the init-migrate-from-openclaw container
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, sts)
				for _, ic := range sts.Spec.Template.Spec.InitContainers {
					if ic.Name == "init-migrate-from-openclaw" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "STS should have init-migrate-from-openclaw init container")
		})

		It("removes init-migrate-from-openclaw once status.migration.completed=true", func() {
			inst := newMigrationInstanceOpenClaw()
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for STS to be created
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// Latch migration as completed via status subresource
			fresh := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, fresh)).To(Succeed())
			fresh.Status.Migration.Completed = true
			Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

			// The controller should now omit the migration init container
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, sts)
				for _, ic := range sts.Spec.Template.Spec.InitContainers {
					if ic.Name == "init-migrate-from-openclaw" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeFalse(), "init-migrate-from-openclaw should be removed after migration completed")
		})

		It("sets ConditionMigrationCompleted=True after status.migration.completed latch", func() {
			inst := newMigrationInstanceOpenClaw()
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for reconciler to process once
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// Set migration completed in status
			fresh := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, fresh)).To(Succeed())
			fresh.Status.Migration.Completed = true
			Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

			// The controller should set ConditionMigrationCompleted=True
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: mName, Namespace: namespace}, fresh)
				return meta.IsStatusConditionTrue(fresh.Status.Conditions, hermesv1.ConditionMigrationCompleted)
			}, timeout, interval).Should(BeTrue(), "ConditionMigrationCompleted should be True after latch")
		})
	})
})
