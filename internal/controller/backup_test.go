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
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"
)

var _ = Describe("Backup sub-controller", func() {
	const (
		bName     = "demo-backup"
		namespace = "default"
		timeout   = 30 * time.Second
		interval  = 250 * time.Millisecond
	)

	ctx := context.Background()

	newBackupInstance := func(schedule string) *hermesv1.HermesInstance {
		inst := &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bName,
				Namespace: namespace,
			},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        "1.0.0",
				},
				Backup: hermesv1.BackupSpec{
					Schedule: schedule,
					S3: &hermesv1.BackupS3Spec{
						Bucket:   "test-bucket",
						Endpoint: "https://s3.example.com",
						CredentialsSecretRef: hermesv1.LocalObjectReference{
							Name: "backup-creds",
						},
					},
				},
			},
		}
		return inst
	}

	AfterEach(func() {
		inst := &hermesv1.HermesInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: bName, Namespace: namespace}, inst); err == nil {
			// Strip any finalizers so deletion isn't blocked by the backup-on-delete finalizer
			if len(inst.Finalizers) > 0 {
				original := inst.DeepCopy()
				inst.Finalizers = nil
				_ = k8sClient.Patch(ctx, inst, client.MergeFrom(original))
			}
			_ = k8sClient.Delete(ctx, inst)
		}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: bName, Namespace: namespace}, inst)
		}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))
	})

	Context("CronJob lifecycle", func() {
		It("creates a backup CronJob when schedule is set", func() {
			inst := newBackupInstance("0 2 * * *")
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			cronJobName := resources.BackupCronJobName(inst)
			cj := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: namespace}, cj)
			}, timeout, interval).Should(Succeed())

			Expect(cj.Spec.Schedule).To(Equal("0 2 * * *"))
		})

		It("removes the backup CronJob when schedule is cleared", func() {
			inst := newBackupInstance("0 2 * * *")
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			cronJobName := resources.BackupCronJobName(inst)
			cj := &batchv1.CronJob{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: namespace}, cj)
			}, timeout, interval).Should(Succeed())

			// Now clear the schedule
			fresh := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: bName, Namespace: namespace}, fresh)).To(Succeed())
			original := fresh.DeepCopy()
			fresh.Spec.Backup.Schedule = ""
			fresh.Spec.Backup.S3 = nil
			Expect(k8sClient.Patch(ctx, fresh, client.MergeFrom(original))).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: namespace}, cj)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Finalizer via Patch (lesson #437 canary)", func() {
		It("adding finalizer via Patch does NOT bump metadata.generation", func() {
			inst := newBackupInstance("")
			inst.Spec.Backup.OnDelete = true
			inst.Spec.Backup.S3 = &hermesv1.BackupS3Spec{
				Bucket:   "test-bucket",
				Endpoint: "https://s3.example.com",
				CredentialsSecretRef: hermesv1.LocalObjectReference{
					Name: "backup-creds",
				},
			}
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for the controller to settle and possibly add a finalizer
			time.Sleep(1 * time.Second)

			// Verify that after reconcile the generation hasn't been bumped
			// by a spurious Update on metadata (only Status and Spec updates bump generation)
			fetched := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: bName, Namespace: namespace}, fetched)).To(Succeed())
			// generation should remain at 1 (created) — a Patch on finalizers only does NOT bump generation
			Expect(fetched.Generation).To(Equal(int64(1)),
				"lesson #437: finalizer patch must NOT use Update which bumps metadata.generation")
		})
	})
})
