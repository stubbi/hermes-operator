package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"
)

// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;create;update;patch;delete

// BackupReconciler is a sub-controller invoked by HermesInstanceReconciler.
// Not a controller-runtime Reconciler: the main reconciler drives it.
type BackupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// EnsureFinalizer adds the backup-on-delete finalizer when spec.backup.onDelete is true.
//
// CRITICAL: lesson #437: finalizer mutation uses r.Patch(ctx, inst, client.MergeFrom(original)),
// NEVER r.Update: Update bumps metadata.generation and triggers a pod-replace.
func (b *BackupReconciler) EnsureFinalizer(ctx context.Context, inst *hermesv1.HermesInstance) error {
	if !inst.Spec.Backup.OnDelete {
		return nil
	}
	if controllerutil.ContainsFinalizer(inst, hermesv1.FinalizerBackupOnDelete) {
		return nil
	}
	original := inst.DeepCopy()
	controllerutil.AddFinalizer(inst, hermesv1.FinalizerBackupOnDelete)
	if err := b.Patch(ctx, inst, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch finalizer add: %w", err)
	}
	return nil
}

// RemoveFinalizer removes the backup-on-delete finalizer via r.Patch (NOT r.Update).
func (b *BackupReconciler) RemoveFinalizer(ctx context.Context, inst *hermesv1.HermesInstance) error {
	if !controllerutil.ContainsFinalizer(inst, hermesv1.FinalizerBackupOnDelete) {
		return nil
	}
	original := inst.DeepCopy()
	controllerutil.RemoveFinalizer(inst, hermesv1.FinalizerBackupOnDelete)
	if err := b.Patch(ctx, inst, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch finalizer remove: %w", err)
	}
	return nil
}

// ReconcileCronJob creates/updates/deletes the periodic backup CronJob based on spec.backup.schedule.
func (b *BackupReconciler) ReconcileCronJob(ctx context.Context, inst *hermesv1.HermesInstance) error {
	if inst.Spec.Backup.Schedule == "" || inst.Spec.Backup.S3 == nil {
		return b.deleteCronJob(ctx, inst, resources.BackupCronJobName(inst))
	}

	obj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.BackupCronJobName(inst),
		Namespace: inst.Namespace,
	}}
	_, err := controllerutil.CreateOrUpdate(ctx, b.Client, obj, func() error {
		desired := resources.BuildBackupCronJob(inst)
		obj.Labels = resources.MergePreservingForeign(obj.Labels, desired.Labels, "hermes.agent/")
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, obj, b.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile backup CronJob: %w", err)
	}

	prune := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{
		Name:      resources.BackupPruneCronJobName(inst),
		Namespace: inst.Namespace,
	}}
	_, err = controllerutil.CreateOrUpdate(ctx, b.Client, prune, func() error {
		desired := resources.BuildBackupPruneCronJob(inst)
		prune.Labels = resources.MergePreservingForeign(prune.Labels, desired.Labels, "hermes.agent/")
		prune.Spec = desired.Spec
		return controllerutil.SetControllerReference(inst, prune, b.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile prune CronJob: %w", err)
	}

	meta.SetStatusCondition(&inst.Status.Conditions, metav1.Condition{
		Type:               hermesv1.ConditionBackupReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Scheduled",
		Message:            fmt.Sprintf("Backup CronJob %q scheduled %q", obj.Name, inst.Spec.Backup.Schedule),
		ObservedGeneration: inst.Generation,
	})
	return nil
}

func (b *BackupReconciler) deleteCronJob(ctx context.Context, inst *hermesv1.HermesInstance, name string) error {
	cj := &batchv1.CronJob{}
	err := b.Get(ctx, types.NamespacedName{Name: name, Namespace: inst.Namespace}, cj)
	if apierrors.IsNotFound(err) {
		meta.RemoveStatusCondition(&inst.Status.Conditions, hermesv1.ConditionBackupReady)
		return nil
	}
	if err != nil {
		return err
	}
	if err := b.Delete(ctx, cj); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	meta.RemoveStatusCondition(&inst.Status.Conditions, hermesv1.ConditionBackupReady)
	return nil
}

// HandleDeletion runs the backup-on-delete state machine.
// Returns (Result, finalizerStillHeld, error). When finalizerStillHeld=true the caller must requeue.
func (b *BackupReconciler) HandleDeletion(ctx context.Context, inst *hermesv1.HermesInstance) (ctrl.Result, bool, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(inst, hermesv1.FinalizerBackupOnDelete) {
		return ctrl.Result{}, false, nil
	}

	if inst.Annotations[hermesv1.AnnotationSkipFinalBackup] == "true" {
		b.Recorder.Eventf(inst, corev1.EventTypeWarning, "FinalBackupSkipped",
			"Skipping final backup because annotation %q is true", hermesv1.AnnotationSkipFinalBackup)
		if err := b.RemoveFinalizer(ctx, inst); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{}, false, nil
	}

	if inst.Spec.Backup.S3 == nil {
		b.Recorder.Eventf(inst, corev1.EventTypeWarning, "FinalBackupSkipped",
			"spec.backup.s3 is unset; cannot run final backup")
		if err := b.RemoveFinalizer(ctx, inst); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{}, false, nil
	}

	jobName := FinalBackupJobName(inst)
	job, err := GetJob(ctx, b.Client, jobName, inst.Namespace)
	if err != nil {
		return ctrl.Result{}, true, err
	}

	if job == nil {
		ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
		key := SnapshotKey(inst, "onDelete", ts)
		desired := resources.BuildBackupOneShotJob(inst, resources.BackupJobOpts{
			Name:        jobName,
			SnapshotKey: key,
			Kind:        "onDelete",
		})
		if err := controllerutil.SetControllerReference(inst, desired, b.Scheme); err != nil {
			return ctrl.Result{}, true, err
		}
		if err := b.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) { // reconcile-guard:allow: final backup Job is create-only
			return ctrl.Result{}, true, fmt.Errorf("create final backup Job: %w", err)
		}
		inst.Status.Backup.FinalBackupJobName = jobName
		if err := b.Status().Update(ctx, inst); err != nil {
			return ctrl.Result{}, true, err
		}
		b.Recorder.Eventf(inst, corev1.EventTypeNormal, "FinalBackupStarted",
			"Final backup Job %q started; snapshot key %q", jobName, key)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, true, nil
	}

	finished, cond := IsJobFinished(job)
	if !finished {
		logger.Info("final backup still running", "job", jobName)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, true, nil
	}

	if cond == batchv1.JobFailed {
		b.Recorder.Eventf(inst, corev1.EventTypeWarning, "FinalBackupFailed",
			"Final backup Job %q failed. Inspect logs, delete the Job to retry, or annotate %q=true to skip.",
			jobName, hermesv1.AnnotationSkipFinalBackup)
		now := metav1.Now()
		inst.Status.Backup.LastFailureTime = &now
		inst.Status.Backup.LastFailureReason = "FinalBackupJobFailed"
		if err := b.Status().Update(ctx, inst); err != nil {
			return ctrl.Result{}, true, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, true, nil
	}

	now := metav1.Now()
	inst.Status.Backup.LastSuccessTime = &now
	if err := b.Status().Update(ctx, inst); err != nil {
		return ctrl.Result{}, true, err
	}
	if err := b.RemoveFinalizer(ctx, inst); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{}, false, nil
}

// RunOneShot creates a one-shot pre-update backup Job and waits for it across reconciles.
// Returns (snapshotKey, done, err). Called by the auto-update controller.
func (b *BackupReconciler) RunOneShot(ctx context.Context, inst *hermesv1.HermesInstance) (string, bool, error) {
	jobName := PreUpdateBackupJobName(inst)
	ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	key := SnapshotKey(inst, "preUpdate", ts)

	job, err := GetJob(ctx, b.Client, jobName, inst.Namespace)
	if err != nil {
		return "", false, err
	}
	if job == nil {
		if inst.Status.AutoUpdate.PreUpdateSnapshot == "" {
			inst.Status.AutoUpdate.PreUpdateSnapshot = key
			if err := b.Status().Update(ctx, inst); err != nil {
				return "", false, err
			}
		} else {
			key = inst.Status.AutoUpdate.PreUpdateSnapshot
		}

		desired := resources.BuildBackupOneShotJob(inst, resources.BackupJobOpts{
			Name:        jobName,
			SnapshotKey: key,
			Kind:        "preUpdate",
		})
		if err := controllerutil.SetControllerReference(inst, desired, b.Scheme); err != nil {
			return "", false, err
		}
		if err := b.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) { // reconcile-guard:allow: pre-update backup Job is create-only
			return "", false, fmt.Errorf("create pre-update backup Job: %w", err)
		}
		b.Recorder.Eventf(inst, corev1.EventTypeNormal, "PreUpdateBackupStarted",
			"Pre-update backup Job %q started; snapshot %q", jobName, key)
		return key, false, nil
	}

	finished, cond := IsJobFinished(job)
	if !finished {
		return inst.Status.AutoUpdate.PreUpdateSnapshot, false, nil
	}
	if cond == batchv1.JobFailed {
		return inst.Status.AutoUpdate.PreUpdateSnapshot, false, fmt.Errorf("pre-update backup Job %q failed", jobName)
	}
	return inst.Status.AutoUpdate.PreUpdateSnapshot, true, nil
}
