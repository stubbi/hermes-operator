package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

func migrationInstanceWithOpenClawRef() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.0.0"},
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Mode: "copy",
					Source: hermesv1.MigrationFromOpenClawSource{
						OpenClawInstanceRef: &hermesv1.NamespacedObjectReference{
							Name:      "my-openclaw",
							Namespace: "agents",
						},
					},
				},
			},
		},
	}
}

func migrationInstanceWithS3() *hermesv1.HermesInstance {
	return &hermesv1.HermesInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agents"},
		Spec: hermesv1.HermesInstanceSpec{
			Image: hermesv1.ImageSpec{Repository: "ghcr.io/stubbi/hermes-agent", Tag: "1.0.0"},
			Migration: hermesv1.MigrationSpec{
				FromOpenClaw: &hermesv1.MigrationFromOpenClawSpec{
					Mode: "copy",
					Source: hermesv1.MigrationFromOpenClawSource{
						BackupRef: &hermesv1.MigrationBackupRef{
							S3: hermesv1.MigrationBackupS3{
								Bucket:               "openclaw-backups",
								Endpoint:             "s3.amazonaws.com",
								Region:               "us-east-1",
								Key:                  "prod/my-openclaw/2026-05-11.tar.zst",
								CredentialsSecretRef: hermesv1.LocalObjectReference{Name: "oc-s3-creds"},
							},
						},
					},
				},
			},
		},
	}
}

func TestBuildMigrationInitContainer_NilWhenNoSpec(t *testing.T) {
	inst := &hermesv1.HermesInstance{}
	assert.Nil(t, BuildMigrationInitContainer(inst))
}

func TestBuildMigrationInitContainer_NilWhenCompleted(t *testing.T) {
	inst := migrationInstanceWithOpenClawRef()
	inst.Status.Migration.Completed = true
	assert.Nil(t, BuildMigrationInitContainer(inst))
}

func TestBuildMigrationInitContainer_OpenClawRef_Name(t *testing.T) {
	c := BuildMigrationInitContainer(migrationInstanceWithOpenClawRef())
	require.NotNil(t, c)
	assert.Equal(t, "init-migrate-from-openclaw", c.Name)
	assert.Equal(t, "ghcr.io/stubbi/hermes-agent:1.0.0", c.Image)
}

func TestBuildMigrationInitContainer_OpenClawRef_Args(t *testing.T) {
	c := BuildMigrationInitContainer(migrationInstanceWithOpenClawRef())
	joined := strings.Join(c.Args, " ")
	assert.Contains(t, joined, "hermes-agent migrate from-openclaw")
	assert.Contains(t, joined, "--source /mnt/openclaw")
	assert.Contains(t, joined, "--dest /home/hermes/.hermes")
}

func TestBuildMigrationInitContainer_OpenClawRef_VolumeMount(t *testing.T) {
	c := BuildMigrationInitContainer(migrationInstanceWithOpenClawRef())
	found := map[string]string{}
	for _, m := range c.VolumeMounts {
		found[m.Name] = m.MountPath
	}
	assert.Equal(t, "/mnt/openclaw", found["openclaw-source"])
	assert.Equal(t, "/home/hermes/.hermes", found["data"])
}

func TestBuildMigrationInitContainer_S3_DownloadsBeforeMigrate(t *testing.T) {
	c := BuildMigrationInitContainer(migrationInstanceWithS3())
	joined := strings.Join(c.Args, " ")
	assert.Contains(t, joined, "prod/my-openclaw/2026-05-11.tar.zst")
	assert.Contains(t, joined, "/mnt/openclaw")
	assert.Contains(t, joined, "hermes-agent migrate from-openclaw")
}

func TestBuildMigrationInitContainer_S3_EnvFromSecret(t *testing.T) {
	c := BuildMigrationInitContainer(migrationInstanceWithS3())
	require.Len(t, c.EnvFrom, 1)
	require.NotNil(t, c.EnvFrom[0].SecretRef)
	assert.Equal(t, "oc-s3-creds", c.EnvFrom[0].SecretRef.Name)
}

func TestBuildMigrationInitContainer_CustomImage(t *testing.T) {
	inst := migrationInstanceWithOpenClawRef()
	inst.Spec.Migration.FromOpenClaw.Image = "internal.registry/hermes-agent:migrate"
	c := BuildMigrationInitContainer(inst)
	assert.Equal(t, "internal.registry/hermes-agent:migrate", c.Image)
}
