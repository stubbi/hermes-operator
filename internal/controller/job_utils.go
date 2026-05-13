package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func FinalBackupJobName(inst *hermesv1.HermesInstance) string { return inst.Name + "-backup-final" }
func PreUpdateBackupJobName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-backup-preupdate"
}
func RestoreJobName(inst *hermesv1.HermesInstance) string   { return inst.Name + "-restore" }
func MigrationJobName(inst *hermesv1.HermesInstance) string { return inst.Name + "-migrate" }

// IsJobFinished reports whether the Job has a terminal condition.
func IsJobFinished(job *batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

// GetJob fetches a Job, returning (nil, nil) on NotFound and (nil, err) otherwise.
func GetJob(ctx context.Context, c client.Client, name, namespace string) (*batchv1.Job, error) {
	j := &batchv1.Job{}
	if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, j); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return j, nil
}
