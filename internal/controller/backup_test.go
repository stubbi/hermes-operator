/*
Copyright 2026 Paperclip.inc.

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

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
	"github.com/paperclipinc/hermes-operator/internal/resources"
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
		key := types.NamespacedName{Name: bName, Namespace: namespace}
		Eventually(func() error {
			inst := &hermesv1.HermesInstance{}
			if err := k8sClient.Get(ctx, key, inst); err != nil {
				return client.IgnoreNotFound(err)
			}
			// Envtest has no Job controller to finish a final backup. Use the
			// supported skip path instead of racing EnsureFinalizer by stripping
			// a finalizer while backup.onDelete is still true.
			original := inst.DeepCopy()
			if inst.Annotations == nil {
				inst.Annotations = make(map[string]string)
			}
			inst.Annotations[hermesv1.AnnotationSkipFinalBackup] = "true"
			if err := k8sClient.Patch(ctx, inst, client.MergeFrom(original)); err != nil {
				return err
			}
			return client.IgnoreNotFound(k8sClient.Delete(ctx, inst))
		}, timeout, interval).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &hermesv1.HermesInstance{})
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

			// Observe the finalizer before checking its metadata-only write.
			Eventually(func(g Gomega) {
				fetched := &hermesv1.HermesInstance{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: bName, Namespace: namespace}, fetched)).To(Succeed())
				g.Expect(fetched.Finalizers).To(ContainElement(hermesv1.FinalizerBackupOnDelete))
				g.Expect(fetched.Generation).To(Equal(int64(1)),
					"lesson #437: finalizer patch must not bump metadata.generation")
			}, timeout, interval).Should(Succeed())
		})
	})
})
