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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// HermesClusterDefaultsReconciler enforces the singleton invariant on the
// cluster-scoped HermesClusterDefaults resource and surfaces Ready status.
type HermesClusterDefaultsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=hermes.agent,resources=hermesclusterdefaults,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=hermes.agent,resources=hermesclusterdefaults/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *HermesClusterDefaultsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	hcd := &hermesv1.HermesClusterDefaults{}
	if err := r.Get(ctx, req.NamespacedName, hcd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cond := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Singleton",
		Message:            "HermesClusterDefaults singleton is healthy",
		ObservedGeneration: hcd.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}

	if hcd.Name != "cluster" {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InvalidName"
		cond.Message = fmt.Sprintf("name must be \"cluster\" (got %q); this resource is ignored by the defaulting webhook", hcd.Name)
		if r.Recorder != nil {
			r.Recorder.Event(hcd, "Warning", "InvalidName", cond.Message)
		}
	}

	meta.SetStatusCondition(&hcd.Status.Conditions, cond)
	hcd.Status.ObservedGeneration = hcd.Generation
	if err := r.Status().Update(ctx, hcd); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager wires the controller.
func (r *HermesClusterDefaultsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hermesv1.HermesClusterDefaults{}).
		Named("hermesclusterdefaults").
		Complete(r)
}
