package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func backupInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents", UID: "uid-1"},
		Spec: hermesv1.HermesInstanceSpec{
			Backup: hermesv1.BackupSpec{
				S3: &hermesv1.BackupS3Spec{
					Bucket:               "hermes-backups",
					Endpoint:             "s3.amazonaws.com",
					Region:               "us-east-1",
					PathPrefix:           "prod/",
					CredentialsSecretRef: hermesv1.LocalObjectReference{Name: "hermes-s3-creds"},
				},
			},
		},
	}
}

func TestBuildBackupOneShotJob_PinnedImageAndNames(t *testing.T) {
	inst := backupInstance()
	job := BuildBackupOneShotJob(inst, BackupJobOpts{
		Name:        "demo-backup-final",
		SnapshotKey: "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst",
		Kind:        "onDelete",
	})
	assert.Equal(t, "demo-backup-final", job.Name)
	assert.Equal(t, "agents", job.Namespace)
	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	c := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "restic/restic:0.16.4", c.Image)
	assert.Equal(t, corev1.PullIfNotPresent, c.ImagePullPolicy)
}

func TestBuildBackupOneShotJob_CustomImage(t *testing.T) {
	inst := backupInstance()
	inst.Spec.Backup.Image = "internal.registry/restic:custom"
	job := BuildBackupOneShotJob(inst, BackupJobOpts{Name: "demo-backup-final", SnapshotKey: "k", Kind: "onDelete"})
	assert.Equal(t, "internal.registry/restic:custom", job.Spec.Template.Spec.Containers[0].Image)
}

func TestBuildBackupOneShotJob_EmbedsSnapshotKey(t *testing.T) {
	inst := backupInstance()
	job := BuildBackupOneShotJob(inst, BackupJobOpts{
		Name:        "demo-backup-final",
		SnapshotKey: "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst",
		Kind:        "onDelete",
	})
	cmd := strings.Join(job.Spec.Template.Spec.Containers[0].Args, " ")
	assert.Contains(t, cmd, "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst")
	assert.Contains(t, cmd, "/home/hermes/.hermes")
}

func TestBuildBackupOneShotJob_PVCRef(t *testing.T) {
	inst := backupInstance()
	job := BuildBackupOneShotJob(inst, BackupJobOpts{Name: "demo-backup-final", SnapshotKey: "k", Kind: "onDelete"})
	volumes := job.Spec.Template.Spec.Volumes
	require.Len(t, volumes, 1)
	assert.Equal(t, "data", volumes[0].Name)
	require.NotNil(t, volumes[0].PersistentVolumeClaim)
	assert.Equal(t, PVCName(inst), volumes[0].PersistentVolumeClaim.ClaimName)
}

func TestBuildBackupOneShotJob_S3CredsViaEnvFromSecret(t *testing.T) {
	inst := backupInstance()
	job := BuildBackupOneShotJob(inst, BackupJobOpts{Name: "demo-backup-final", SnapshotKey: "k", Kind: "onDelete"})
	c := job.Spec.Template.Spec.Containers[0]
	require.Len(t, c.EnvFrom, 1)
	require.NotNil(t, c.EnvFrom[0].SecretRef)
	assert.Equal(t, "hermes-s3-creds", c.EnvFrom[0].SecretRef.Name)
}

func TestBuildBackupOneShotJob_BackoffAndTTL(t *testing.T) {
	inst := backupInstance()
	job := BuildBackupOneShotJob(inst, BackupJobOpts{Name: "demo-backup-final", SnapshotKey: "k", Kind: "onDelete"})
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(3), *job.Spec.BackoffLimit)
	require.NotNil(t, job.Spec.TTLSecondsAfterFinished)
	assert.Equal(t, int32(86400), *job.Spec.TTLSecondsAfterFinished)
}
