package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"
)

// RestoreReconciler watches the StatefulSet for init-restore completion.
type RestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile checks whether the in-pod init-restore container has finished.
// On success: status.restoredFrom = spec.restoreFrom (terminal latch).
// Returns (Result, done, err). done=true means restore is not in flight.
func (r *RestoreReconciler) Reconcile(ctx context.Context, inst *hermesv1.HermesInstance) (ctrl.Result, bool, error) {
	logger := log.FromContext(ctx)

	if inst.Spec.RestoreFrom == "" {
		return ctrl.Result{}, true, nil
	}
	if inst.Status.RestoredFrom == inst.Spec.RestoreFrom {
		if !meta.IsStatusConditionTrue(inst.Status.Conditions, hermesv1.ConditionRestoreApplied) {
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionRestoreApplied,
				Status:             metav1.ConditionTrue,
				Reason:             "RestoreCompleted",
				Message:            fmt.Sprintf("Restored from %s", inst.Status.RestoredFrom),
				ObservedGeneration: inst.Generation,
			})
		}
		return ctrl.Result{}, true, nil
	}

	podName := resources.StatefulSetName(inst) + "-0"
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: inst.Namespace}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, false, nil
		}
		return ctrl.Result{}, false, err
	}

	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.Name != "init-restore" {
			continue
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0 {
			inst.Status.RestoredFrom = inst.Spec.RestoreFrom
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionRestoreApplied,
				Status:             metav1.ConditionTrue,
				Reason:             "RestoreCompleted",
				Message:            fmt.Sprintf("Restored from %s", inst.Spec.RestoreFrom),
				ObservedGeneration: inst.Generation,
			})
			if err := r.Status().Update(ctx, inst); err != nil {
				return ctrl.Result{}, false, err
			}
			r.Recorder.Eventf(inst, corev1.EventTypeNormal, "RestoreCompleted",
				"Restored from %s", inst.Spec.RestoreFrom)
			return ctrl.Result{}, true, nil
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionRestoreApplied,
				Status:             metav1.ConditionFalse,
				Reason:             "RestoreFailed",
				Message:            fmt.Sprintf("init-restore exited %d: %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Message),
				ObservedGeneration: inst.Generation,
			})
			r.Recorder.Eventf(inst, corev1.EventTypeWarning, "RestoreFailed",
				"init-restore exited %d: %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
			if err := r.Status().Update(ctx, inst); err != nil {
				return ctrl.Result{}, false, err
			}
			return ctrl.Result{}, true, fmt.Errorf("init-restore failed: %s", cs.State.Terminated.Message)
		}
	}

	logger.Info("waiting for init-restore to complete", "pod", podName)
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionRestoreApplied,
		Status:             metav1.ConditionFalse,
		Reason:             "Restoring",
		Message:            fmt.Sprintf("init-restore in progress for %s", inst.Spec.RestoreFrom),
		ObservedGeneration: inst.Generation,
	})
	return ctrl.Result{}, false, nil
}
