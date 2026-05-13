package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func cronInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Backup: hermesv1.BackupSpec{
				S3: &hermesv1.BackupS3Spec{
					Bucket:               "hermes-backups",
					Endpoint:             "s3.amazonaws.com",
					CredentialsSecretRef: hermesv1.LocalObjectReference{Name: "hermes-s3-creds"},
				},
				Schedule: "0 3 * * *",
			},
		},
	}
}

func TestBuildBackupCronJob_BasicShape(t *testing.T) {
	cj := BuildBackupCronJob(cronInstance())
	require.NotNil(t, cj)
	assert.Equal(t, "demo-backup-cron", cj.Name)
	assert.Equal(t, "agents", cj.Namespace)
	assert.Equal(t, "0 3 * * *", cj.Spec.Schedule)
	assert.Equal(t, batchv1.ForbidConcurrent, cj.Spec.ConcurrencyPolicy)
}

func TestBuildBackupCronJob_HistoryLimitsFromSpec(t *testing.T) {
	inst := cronInstance()
	h := int32(7)
	f := int32(2)
	inst.Spec.Backup.HistoryLimit = &h
	inst.Spec.Backup.FailedHistoryLimit = &f
	cj := BuildBackupCronJob(inst)
	require.NotNil(t, cj.Spec.SuccessfulJobsHistoryLimit)
	require.NotNil(t, cj.Spec.FailedJobsHistoryLimit)
	assert.Equal(t, int32(7), *cj.Spec.SuccessfulJobsHistoryLimit)
	assert.Equal(t, int32(2), *cj.Spec.FailedJobsHistoryLimit)
}

func TestBuildBackupCronJob_TemplateUsesPVC(t *testing.T) {
	cj := BuildBackupCronJob(cronInstance())
	vols := cj.Spec.JobTemplate.Spec.Template.Spec.Volumes
	require.Len(t, vols, 1)
	require.NotNil(t, vols[0].PersistentVolumeClaim)
	assert.Equal(t, PVCName(cronInstance()), vols[0].PersistentVolumeClaim.ClaimName)
}

func TestBuildBackupCronJob_CommandIncludesTimestampedKey(t *testing.T) {
	cj := BuildBackupCronJob(cronInstance())
	args := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args
	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "agents/demo")
	assert.Contains(t, joined, "TIMESTAMP")
}

func TestBuildBackupPruneCronJob_RunsDaily(t *testing.T) {
	cj := BuildBackupPruneCronJob(cronInstance())
	require.NotNil(t, cj)
	assert.Equal(t, "demo-backup-prune", cj.Name)
	assert.Equal(t, "17 4 * * *", cj.Spec.Schedule)
}
