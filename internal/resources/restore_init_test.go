package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func restoreInstance() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			RestoreFrom: "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst",
			Backup: hermesv1.BackupSpec{
				S3: &hermesv1.BackupS3Spec{
					Bucket:               "hermes-backups",
					Endpoint:             "s3.amazonaws.com",
					Region:               "us-east-1",
					CredentialsSecretRef: hermesv1.LocalObjectReference{Name: "hermes-s3-creds"},
				},
			},
		},
	}
}

func TestBuildRestoreInitContainer_NameAndImage(t *testing.T) {
	c := BuildRestoreInitContainer(restoreInstance())
	require.NotNil(t, c)
	assert.Equal(t, "init-restore", c.Name)
	assert.Equal(t, "restic/restic:0.16.4", c.Image)
}

func TestBuildRestoreInitContainer_EmbedsSnapshotKey(t *testing.T) {
	c := BuildRestoreInitContainer(restoreInstance())
	joined := strings.Join(c.Args, " ")
	assert.Contains(t, joined, "prod/agents/demo/2026-05-10T03-00-00Z.tar.zst")
	assert.Contains(t, joined, "/home/hermes/.hermes")
}

func TestBuildRestoreInitContainer_SecurityContext(t *testing.T) {
	c := BuildRestoreInitContainer(restoreInstance())
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.AllowPrivilegeEscalation)
	assert.False(t, *c.SecurityContext.AllowPrivilegeEscalation)
	require.NotNil(t, c.SecurityContext.ReadOnlyRootFilesystem)
	assert.True(t, *c.SecurityContext.ReadOnlyRootFilesystem)
}

func TestBuildRestoreInitContainer_S3CredsViaEnvFromSecret(t *testing.T) {
	c := BuildRestoreInitContainer(restoreInstance())
	require.Len(t, c.EnvFrom, 1)
	require.NotNil(t, c.EnvFrom[0].SecretRef)
	assert.Equal(t, "hermes-s3-creds", c.EnvFrom[0].SecretRef.Name)
}

func TestBuildRestoreInitContainer_VolumeMount(t *testing.T) {
	c := BuildRestoreInitContainer(restoreInstance())
	require.Len(t, c.VolumeMounts, 1)
	vm := c.VolumeMounts[0]
	assert.Equal(t, "data", vm.Name)
	assert.Equal(t, "/home/hermes/.hermes", vm.MountPath)
}

func TestBuildRestoreInitContainer_NilWhenNoRestore(t *testing.T) {
	inst := restoreInstance()
	inst.Spec.RestoreFrom = ""
	assert.Nil(t, BuildRestoreInitContainer(inst))
}

func TestBuildRestoreInitContainer_NilWhenAlreadyRestored(t *testing.T) {
	inst := restoreInstance()
	inst.Status.RestoredFrom = inst.Spec.RestoreFrom
	assert.Nil(t, BuildRestoreInitContainer(inst))
}
