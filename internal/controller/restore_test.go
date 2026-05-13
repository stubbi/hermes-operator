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

var _ = Describe("Restore sub-controller", func() {
	const (
		rName     = "demo-restore"
		namespace = "default"
		timeout   = 30 * time.Second
		interval  = 250 * time.Millisecond
	)

	ctx := context.Background()

	newRestoreInstance := func(restoreFrom string) *hermesv1.HermesInstance {
		return &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rName,
				Namespace: namespace,
			},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "1.0.0",
				},
				RestoreFrom: restoreFrom,
				Backup: hermesv1.BackupSpec{
					S3: &hermesv1.BackupS3Spec{
						Bucket:   "test-bucket",
						Endpoint: "https://s3.example.com",
						CredentialsSecretRef: hermesv1.LocalObjectReference{
							Name: "restore-creds",
						},
					},
				},
			},
		}
	}

	AfterEach(func() {
		inst := &hermesv1.HermesInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, inst); err == nil {
			if len(inst.Finalizers) > 0 {
				original := inst.DeepCopy()
				inst.Finalizers = nil
				_ = k8sClient.Patch(ctx, inst, client.MergeFrom(original))
			}
			_ = k8sClient.Delete(ctx, inst)
		}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, inst)
		}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))
	})

	Context("init-restore injection", func() {
		It("injects init-restore into STS PodTemplate when spec.restoreFrom is set", func() {
			inst := newRestoreInstance("my-backup-bucket/snapshots/2026-01-01T00-00-00Z.tar.zst")
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// The controller should create a STS with the init-restore container
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// The STS should have an init-restore init container
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, sts)
				for _, ic := range sts.Spec.Template.Spec.InitContainers {
					if ic.Name == "init-restore" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "STS should have init-restore init container")
		})

		It("removes init-restore once status.restoredFrom matches spec.restoreFrom", func() {
			snapshotKey := "my-bucket/snapshots/2026-02-01T00-00-00Z.tar.zst"
			inst := newRestoreInstance(snapshotKey)
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for STS to be created first
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// Latch the restore by setting status.restoredFrom = spec.restoreFrom
			fresh := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, fresh)).To(Succeed())
			fresh.Status.RestoredFrom = snapshotKey
			Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

			// The controller should now omit the init-restore init container
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, sts)
				for _, ic := range sts.Spec.Template.Spec.InitContainers {
					if ic.Name == "init-restore" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeFalse(), "init-restore should be removed after restore latch")
		})

		It("marks ConditionRestoreApplied=True once status.restoredFrom is latched", func() {
			snapshotKey := "my-bucket/snapshots/2026-03-01T00-00-00Z.tar.zst"
			inst := newRestoreInstance(snapshotKey)
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Latch the restore in status
			fresh := &hermesv1.HermesInstance{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, fresh)
			}, timeout, interval).Should(Succeed())
			fresh.Status.RestoredFrom = snapshotKey
			Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())

			// The controller should set ConditionRestoreApplied=True
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: rName, Namespace: namespace}, fresh)
				return meta.IsStatusConditionTrue(fresh.Status.Conditions, hermesv1.ConditionRestoreApplied)
			}, timeout, interval).Should(BeTrue(), "ConditionRestoreApplied should be True after restore latch")
		})
	})
})
