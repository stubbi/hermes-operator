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

// +kubebuilder:rbac:groups=openclaw.rocks,resources=openclawinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch

// MigrationReconciler watches the StatefulSet pod's init container status and
// latches migration completion. Once Completed=true the validator (Task 19)
// rejects further spec.migration mutations.
type MigrationReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile drives migration completion. Returns (Result, done, err).
// done=true means migration is not in flight.
func (m *MigrationReconciler) Reconcile(ctx context.Context, inst *hermesv1.HermesInstance) (ctrl.Result, bool, error) {
	logger := log.FromContext(ctx)

	if inst.Spec.Migration.FromOpenClaw == nil {
		return ctrl.Result{}, true, nil
	}
	if inst.Status.Migration.Completed {
		if !meta.IsStatusConditionTrue(inst.Status.Conditions, hermesv1.ConditionMigrationCompleted) {
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionMigrationCompleted,
				Status:             metav1.ConditionTrue,
				Reason:             "MigrationCompleted",
				Message:            "OpenClaw -> Hermes migration completed",
				ObservedGeneration: inst.Generation,
			})
		}
		return ctrl.Result{}, true, nil
	}

	podName := resources.StatefulSetName(inst) + "-0"
	pod := &corev1.Pod{}
	if err := m.Get(ctx, types.NamespacedName{Name: podName, Namespace: inst.Namespace}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, false, nil
		}
		return ctrl.Result{}, false, err
	}

	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.Name != "init-migrate-from-openclaw" {
			continue
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0 {
			now := metav1.Now()
			inst.Status.Migration.Completed = true
			inst.Status.Migration.FinishedAt = &now
			inst.Status.Migration.SourceVersion = cs.State.Terminated.Message
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionMigrationCompleted,
				Status:             metav1.ConditionTrue,
				Reason:             "MigrationCompleted",
				Message:            fmt.Sprintf("OpenClaw -> Hermes migration completed at %s", now.Format("2006-01-02T15:04:05Z")),
				ObservedGeneration: inst.Generation,
			})
			if err := m.Status().Update(ctx, inst); err != nil {
				return ctrl.Result{}, false, err
			}
			m.Recorder.Eventf(inst, corev1.EventTypeNormal, "MigrationCompleted",
				"OpenClaw -> Hermes migration completed")

			if inst.Spec.Migration.FromOpenClaw.Mode == "move" &&
				inst.Spec.Migration.FromOpenClaw.Source.OpenClawInstanceRef != nil {
				ref := inst.Spec.Migration.FromOpenClaw.Source.OpenClawInstanceRef
				m.Recorder.Eventf(inst, corev1.EventTypeWarning, "MigrationMoveModeAdvisory",
					"Migration mode is `move`. The operator will NOT delete the source OpenClawInstance %s/%s automatically. "+
						"Once you have verified the migration, run: kubectl -n %s delete openclawinstance %s",
					ref.Namespace, ref.Name, ref.Namespace, ref.Name)
			}
			return ctrl.Result{}, true, nil
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionMigrationCompleted,
				Status:             metav1.ConditionFalse,
				Reason:             "MigrationFailed",
				Message:            fmt.Sprintf("init-migrate-from-openclaw exited %d: %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Message),
				ObservedGeneration: inst.Generation,
			})
			m.Recorder.Eventf(inst, corev1.EventTypeWarning, "MigrationFailed",
				"init-migrate-from-openclaw exited %d: %s", cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
			if err := m.Status().Update(ctx, inst); err != nil {
				return ctrl.Result{}, false, err
			}
			return ctrl.Result{}, true, fmt.Errorf("migration init container failed: %s", cs.State.Terminated.Message)
		}
	}

	logger.Info("waiting for init-migrate-from-openclaw")
	return ctrl.Result{}, false, nil
}

// BuildSourceVolume returns the StatefulSet Volume that mounts the source
// OpenClawInstance's PVC (read-only) or an emptyDir for S3 mode. Returns nil
// when no migration is configured or already completed.
func (m *MigrationReconciler) BuildSourceVolume(inst *hermesv1.HermesInstance) *corev1.Volume {
	if inst.Spec.Migration.FromOpenClaw == nil || inst.Status.Migration.Completed {
		return nil
	}
	ref := inst.Spec.Migration.FromOpenClaw.Source.OpenClawInstanceRef
	if ref == nil {
		return &corev1.Volume{
			Name: resources.MigrationSourceVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	return &corev1.Volume{
		Name: resources.MigrationSourceVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: ref.Name + "-data",
				ReadOnly:  true,
			},
		},
	}
}
