package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
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
	"github.com/stubbi/hermes-operator/internal/oci"
	"github.com/stubbi/hermes-operator/internal/resources"
)

// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

// AutoUpdateReconciler drives OCI-registry polling and rollouts.
type AutoUpdateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Registry oci.Registry
	Backup   *BackupReconciler
	Now      func() time.Time // injectable for tests
}

const rolloutWindow = 5 * time.Minute

func (a *AutoUpdateReconciler) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Reconcile drives the autoupdate state machine for one instance.
func (a *AutoUpdateReconciler) Reconcile(ctx context.Context, inst *hermesv1.HermesInstance) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !inst.Spec.AutoUpdate.Enabled {
		return ctrl.Result{}, nil
	}
	if a.Registry == nil {
		return ctrl.Result{}, nil
	}

	if inst.Status.AutoUpdate.TargetTag != "" {
		return a.driveRollout(ctx, inst)
	}

	interval, err := parsePollInterval(inst.Spec.AutoUpdate.PollInterval)
	if err != nil {
		logger.Error(err, "invalid pollInterval; using 1h")
		interval = time.Hour
	}
	if inst.Status.AutoUpdate.LastCheckTime != nil &&
		a.now().Sub(inst.Status.AutoUpdate.LastCheckTime.Time) < interval {
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	repo := inst.Spec.AutoUpdate.Source.Registry
	if repo == "" {
		repo = inst.Spec.Image.Repository
	}
	tags, err := a.Registry.ListTags(ctx, repo)
	if err != nil {
		a.Recorder.Eventf(inst, corev1.EventTypeWarning, "AutoUpdateListTagsFailed",
			"Could not list tags for %s: %v", repo, err)
		now := metav1.NewTime(a.now())
		inst.Status.AutoUpdate.LastCheckTime = &now
		if statusErr := a.Status().Update(ctx, inst); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	channel := inst.Spec.AutoUpdate.Source.Channel
	if channel == "" {
		channel = oci.DefaultChannel(inst.Spec.Image.Tag)
	}

	best, err := oci.HighestMatching(tags, channel)
	now := metav1.NewTime(a.now())
	inst.Status.AutoUpdate.LastCheckTime = &now
	if err != nil {
		if errors.Is(err, oci.ErrNoMatchingTag) {
			meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
				Type:               hermesv1.ConditionAutoUpdated,
				Status:             metav1.ConditionFalse,
				Reason:             "NoMatchingTag",
				Message:            fmt.Sprintf("no tag in %s matches channel %q", repo, channel),
				ObservedGeneration: inst.Generation,
			})
			if err := a.Status().Update(ctx, inst); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: interval}, nil
		}
		return ctrl.Result{}, fmt.Errorf("HighestMatching: %w", err)
	}

	if best == inst.Status.AutoUpdate.LastFailedTag {
		meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
			Type:               hermesv1.ConditionAutoUpdated,
			Status:             metav1.ConditionFalse,
			Reason:             "SuppressedKnownFailure",
			Message:            fmt.Sprintf("not retrying tag %s (recorded in lastFailedTag)", best),
			ObservedGeneration: inst.Generation,
		})
		if err := a.Status().Update(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	current := currentRunningTag(inst)
	if best == current {
		meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
			Type:               hermesv1.ConditionAutoUpdated,
			Status:             metav1.ConditionTrue,
			Reason:             "UpToDate",
			Message:            fmt.Sprintf("current tag %s is the highest in channel %q", current, channel),
			ObservedGeneration: inst.Generation,
		})
		if err := a.Status().Update(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	return a.startRollout(ctx, inst, best)
}

func currentRunningTag(inst *hermesv1.HermesInstance) string {
	if inst.Status.AutoUpdate.CurrentTag != "" {
		return inst.Status.AutoUpdate.CurrentTag
	}
	return inst.Spec.Image.Tag
}

func (a *AutoUpdateReconciler) startRollout(ctx context.Context, inst *hermesv1.HermesInstance, targetTag string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	needsBackup := inst.Spec.AutoUpdate.BackupBeforeUpdate == nil || *inst.Spec.AutoUpdate.BackupBeforeUpdate
	if needsBackup && inst.Spec.Backup.S3 != nil && a.Backup != nil {
		_, done, err := a.Backup.RunOneShot(ctx, inst)
		if err != nil {
			a.Recorder.Eventf(inst, corev1.EventTypeWarning, "AutoUpdatePreUpdateBackupFailed",
				"pre-update backup failed: %v: aborting rollout", err)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		if !done {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	sts := &appsv1.StatefulSet{}
	if err := a.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	repo := inst.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/stubbi/hermes-agent"
	}
	desiredImage := fmt.Sprintf("%s:%s", repo, targetTag)
	if len(sts.Spec.Template.Spec.Containers) == 0 {
		return ctrl.Result{}, errors.New("StatefulSet has no containers")
	}
	if sts.Spec.Template.Spec.Containers[0].Image == desiredImage {
		logger.Info("STS already at target image; skipping patch", "target", targetTag)
	} else {
		original := sts.DeepCopy()
		sts.Spec.Template.Spec.Containers[0].Image = desiredImage
		if err := a.Patch(ctx, sts, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch STS image: %w", err)
		}
	}

	origCR := inst.DeepCopy()
	if inst.Annotations == nil {
		inst.Annotations = map[string]string{}
	}
	inst.Annotations[hermesv1.AnnotationAutoUpdateTarget] = targetTag
	if err := a.Patch(ctx, inst, client.MergeFrom(origCR)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch CR annotation: %w", err)
	}

	deadline := metav1.NewTime(a.now().Add(rolloutWindow))
	inst.Status.AutoUpdate.TargetTag = targetTag
	inst.Status.AutoUpdate.RolloutDeadline = &deadline
	inst.Status.AutoUpdate.ProbeFailures = 0
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionAutoUpdated,
		Status:             metav1.ConditionFalse,
		Reason:             "RolloutInFlight",
		Message:            fmt.Sprintf("rolling out tag %s", targetTag),
		ObservedGeneration: inst.Generation,
	})
	if err := a.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}
	a.Recorder.Eventf(inst, corev1.EventTypeNormal, "AutoUpdateStarted",
		"rolling out tag %s (deadline %s)", targetTag, deadline.Format(time.RFC3339))
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (a *AutoUpdateReconciler) driveRollout(ctx context.Context, inst *hermesv1.HermesInstance) (ctrl.Result, error) {
	target := inst.Status.AutoUpdate.TargetTag

	sts := &appsv1.StatefulSet{}
	if err := a.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		return ctrl.Result{}, err
	}

	if sts.Status.ReadyReplicas == 1 && sts.Status.UpdatedReplicas == 1 && sts.Status.CurrentRevision != "" {
		return a.confirmRollout(ctx, inst, target)
	}

	failures, err := a.countProbeFailures(ctx, inst)
	if err != nil {
		return ctrl.Result{}, err
	}
	inst.Status.AutoUpdate.ProbeFailures = failures

	threshold := int32(3)
	if inst.Spec.AutoUpdate.Rollback.ProbeFailureThreshold > 0 {
		threshold = inst.Spec.AutoUpdate.Rollback.ProbeFailureThreshold
	}
	rollbackEnabled := inst.Spec.AutoUpdate.Rollback.Enabled == nil || *inst.Spec.AutoUpdate.Rollback.Enabled

	pastDeadline := inst.Status.AutoUpdate.RolloutDeadline != nil &&
		a.now().After(inst.Status.AutoUpdate.RolloutDeadline.Time)

	if rollbackEnabled && (failures >= threshold || pastDeadline) {
		return a.rollback(ctx, inst, target,
			fmt.Sprintf("probe failures %d >= threshold %d (pastDeadline=%v)", failures, threshold, pastDeadline))
	}

	if err := a.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

func (a *AutoUpdateReconciler) confirmRollout(ctx context.Context, inst *hermesv1.HermesInstance, target string) (ctrl.Result, error) {
	inst.Status.AutoUpdate.LastSuccessTag = target
	inst.Status.AutoUpdate.CurrentTag = target
	inst.Status.AutoUpdate.TargetTag = ""
	inst.Status.AutoUpdate.RolloutDeadline = nil
	inst.Status.AutoUpdate.ProbeFailures = 0
	inst.Status.AutoUpdate.PreUpdateSnapshot = ""
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionAutoUpdated,
		Status:             metav1.ConditionTrue,
		Reason:             "Confirmed",
		Message:            fmt.Sprintf("tag %s confirmed at %s", target, a.now().Format(time.RFC3339)),
		ObservedGeneration: inst.Generation,
	})
	meta.RemoveStatusCondition(&inst.Status.Conditions, hermesv1.ConditionAutoUpdateRolledBack)
	if err := a.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}

	if inst.Annotations[hermesv1.AnnotationAutoUpdateTarget] != "" {
		original := inst.DeepCopy()
		delete(inst.Annotations, hermesv1.AnnotationAutoUpdateTarget)
		if err := a.Patch(ctx, inst, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, err
		}
	}

	a.Recorder.Eventf(inst, corev1.EventTypeNormal, "AutoUpdateConfirmed",
		"tag %s rolled out and passed readiness watch", target)
	return ctrl.Result{}, nil
}

func (a *AutoUpdateReconciler) rollback(ctx context.Context, inst *hermesv1.HermesInstance, failedTag, reason string) (ctrl.Result, error) {
	prev := inst.Status.AutoUpdate.LastSuccessTag
	if prev == "" {
		prev = inst.Spec.Image.Tag
	}
	repo := inst.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/stubbi/hermes-agent"
	}
	sts := &appsv1.StatefulSet{}
	if err := a.Get(ctx, types.NamespacedName{Name: resources.StatefulSetName(inst), Namespace: inst.Namespace}, sts); err != nil {
		return ctrl.Result{}, err
	}
	original := sts.DeepCopy()
	if len(sts.Spec.Template.Spec.Containers) == 0 {
		return ctrl.Result{}, errors.New("STS has no containers")
	}
	sts.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", repo, prev)
	if err := a.Patch(ctx, sts, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch STS rollback: %w", err)
	}

	inst.Status.AutoUpdate.LastFailedTag = failedTag
	inst.Status.AutoUpdate.TargetTag = ""
	inst.Status.AutoUpdate.RolloutDeadline = nil
	inst.Status.AutoUpdate.CurrentTag = prev
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionAutoUpdateRolledBack,
		Status:             metav1.ConditionTrue,
		Reason:             fmt.Sprintf("RolledBackFrom_%s", failedTag),
		Message:            reason,
		ObservedGeneration: inst.Generation,
	})
	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionAutoUpdated,
		Status:             metav1.ConditionFalse,
		Reason:             "RolledBack",
		Message:            fmt.Sprintf("tag %s failed readiness; reverted to %s", failedTag, prev),
		ObservedGeneration: inst.Generation,
	})
	if err := a.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, err
	}

	if inst.Annotations[hermesv1.AnnotationAutoUpdateTarget] != "" {
		origCR := inst.DeepCopy()
		delete(inst.Annotations, hermesv1.AnnotationAutoUpdateTarget)
		if err := a.Patch(ctx, inst, client.MergeFrom(origCR)); err != nil {
			return ctrl.Result{}, err
		}
	}

	a.Recorder.Eventf(inst, corev1.EventTypeWarning, "AutoUpdateRolledBack",
		"rolled back from %s to %s: %s", failedTag, prev, reason)
	return ctrl.Result{}, nil
}

// countProbeFailures counts Unhealthy / FailedMount events on the instance's pod within the rollout window.
func (a *AutoUpdateReconciler) countProbeFailures(ctx context.Context, inst *hermesv1.HermesInstance) (int32, error) {
	list := &corev1.EventList{}
	if err := a.List(ctx, list, client.InNamespace(inst.Namespace)); err != nil {
		return 0, err
	}
	podName := resources.StatefulSetName(inst) + "-0"
	var count int32
	for _, e := range list.Items {
		if e.InvolvedObject.Name != podName || e.InvolvedObject.Kind != "Pod" {
			continue
		}
		if e.Reason != "Unhealthy" && e.Reason != "FailedMount" {
			continue
		}
		if inst.Status.AutoUpdate.RolloutDeadline != nil {
			windowStart := inst.Status.AutoUpdate.RolloutDeadline.Add(-rolloutWindow)
			if e.LastTimestamp.Time.Before(windowStart) {
				continue
			}
		}
		count++
	}
	return count, nil
}

func parsePollInterval(s string) (time.Duration, error) {
	if s == "" {
		return time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < 15*time.Minute {
		return 15 * time.Minute, nil
	}
	if d > 168*time.Hour {
		return 168 * time.Hour, nil
	}
	return d, nil
}
