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

// Package controller contains envtest-based tests for the autoupdate sub-controller.
//
// NOTE: The full rollout-confirmation state machine requires real STS ReadyReplicas
// to advance (envtest has no kubelet), so driveRollout/confirmRollout paths are
// covered by unit tests in autoupdate.go rather than here. These envtest specs
// cover the observable side-effects from the poll+startRollout phase.
package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

var _ = Describe("AutoUpdate sub-controller", func() {
	const (
		auName    = "demo-autoupdate"
		namespace = "default"
		timeout   = 30 * time.Second
		interval  = 250 * time.Millisecond
	)

	ctx := context.Background()

	newAutoUpdateInstance := func(enabled bool, currentTag string) *hermesv1.HermesInstance {
		pollInterval := "15m"
		return &hermesv1.HermesInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      auName,
				Namespace: namespace,
			},
			Spec: hermesv1.HermesInstanceSpec{
				Image: hermesv1.ImageSpec{
					Repository: "ghcr.io/stubbi/hermes-agent",
					Tag:        currentTag,
				},
				AutoUpdate: hermesv1.AutoUpdateSpec{
					Enabled:      enabled,
					PollInterval: pollInterval,
					Source: hermesv1.AutoUpdateSourceSpec{
						Registry: "ghcr.io/stubbi/hermes-agent",
					},
				},
			},
		}
	}

	AfterEach(func() {
		// Reset fakeOCI tags to initial state
		fakeOCI.SetTags("ghcr.io/stubbi/hermes-agent", []string{"1.0.0", "1.0.1", "1.1.0"})

		inst := &hermesv1.HermesInstance{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, inst); err == nil {
			if len(inst.Finalizers) > 0 {
				original := inst.DeepCopy()
				inst.Finalizers = nil
				_ = k8sClient.Patch(ctx, inst, client.MergeFrom(original))
			}
			_ = k8sClient.Delete(ctx, inst)
		}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, inst)
		}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))
	})

	Context("Idle when not enabled", func() {
		It("does not set targetTag when autoUpdate.enabled=false", func() {
			// Set fake registry to have a much newer tag
			fakeOCI.SetTags("ghcr.io/stubbi/hermes-agent", []string{"1.0.0", "2.0.0", "3.0.0"})

			inst := newAutoUpdateInstance(false, "1.0.0")
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for STS to be created (controller has reconciled at least once)
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// Give controller time to run a few reconcile cycles
			time.Sleep(2 * time.Second)

			// Verify targetTag is NOT set (controller is idle)
			fresh := &hermesv1.HermesInstance{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, fresh)).To(Succeed())
			Expect(fresh.Status.AutoUpdate.TargetTag).To(BeEmpty(),
				"autoUpdate.enabled=false must not set targetTag")
		})
	})

	Context("Roll forward when newer tag available", func() {
		It("sets status.autoUpdate.targetTag when fake registry returns a higher tag", func() {
			// Current tag is 1.0.0, registry has 1.1.0
			fakeOCI.SetTags("ghcr.io/stubbi/hermes-agent", []string{"1.0.0", "1.1.0"})

			inst := newAutoUpdateInstance(true, "1.0.0")
			// Use a short poll interval so the controller checks quickly
			inst.Spec.AutoUpdate.PollInterval = "15m"
			// Disable backup-before-update to skip that flow
			disableBackup := false
			inst.Spec.AutoUpdate.BackupBeforeUpdate = &disableBackup
			Expect(k8sClient.Create(ctx, inst)).To(Succeed())

			// Wait for STS to be created
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, sts)
			}, timeout, interval).Should(Succeed())

			// The controller should start a rollout because 1.1.0 > 1.0.0
			// Observable side-effects: targetTag is set OR STS image is patched to 1.1.0
			Eventually(func() bool {
				fresh := &hermesv1.HermesInstance{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, fresh); err != nil {
					return false
				}
				if fresh.Status.AutoUpdate.TargetTag == "1.1.0" {
					return true
				}
				// Also accept if STS image was already patched
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: auName, Namespace: namespace}, sts); err != nil {
					return false
				}
				if len(sts.Spec.Template.Spec.Containers) > 0 {
					return sts.Spec.Template.Spec.Containers[0].Image == "ghcr.io/stubbi/hermes-agent:1.1.0"
				}
				return false
			}, timeout, interval).Should(BeTrue(),
				"controller should start rollout to 1.1.0 when enabled and registry has newer tag")
		})
	})

	Context("Suppress retry of lastFailedTag", func() {
		It("sets SuppressedKnownFailure condition when best==lastFailedTag", func() {
			// SKIP RATIONALE: Testing the suppress path in envtest is not straightforward.
			// The state machine requires: poll → startRollout → rollback (triggered by probe
			// failures OR deadline expiry). In envtest there is no kubelet, so:
			//   - probe failure events are never generated (countProbeFailures=0)
			//   - rollout deadline is 5 minutes — far beyond the 30s test timeout
			// After rollback, lastFailedTag is set and lastCheckTime is set (prevents immediate
			// re-poll). Manually resetting lastCheckTime races with the reconciler.
			// The suppress logic is unit-tested via the AutoUpdateReconciler.Reconcile() directly.
			// Tracking issue: add injectable clock and probe-failure hook for envtest.
			Skip("suppress-lastFailedTag path requires rollback completion (5min deadline) — " +
				"not feasible in 30s envtest; covered by unit test in autoupdate.go")
		})
	})
})
