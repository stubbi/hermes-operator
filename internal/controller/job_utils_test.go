package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
	"github.com/stubbi/hermes-operator/internal/resources"
)

func TestJobNames(t *testing.T) {
	inst := &hermesv1.HermesInstance{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"}}
	assert.Equal(t, "demo-backup-final", FinalBackupJobName(inst))
	assert.Equal(t, "demo-backup-preupdate", PreUpdateBackupJobName(inst))
	assert.Equal(t, "demo-restore", RestoreJobName(inst))
	assert.Equal(t, "demo-backup-cron", resources.BackupCronJobName(inst))
	assert.Equal(t, "demo-backup-prune", resources.BackupPruneCronJobName(inst))
}

func TestIsJobFinished(t *testing.T) {
	j := &batchv1.Job{}
	finished, _ := IsJobFinished(j)
	assert.False(t, finished)

	j.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
	}}
	finished, cond := IsJobFinished(j)
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobComplete, cond)

	j.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
	}}
	finished, cond = IsJobFinished(j)
	assert.True(t, finished)
	assert.Equal(t, batchv1.JobFailed, cond)
}
