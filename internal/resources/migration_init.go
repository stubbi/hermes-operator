package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// MigrationSourceVolumeName is the volume name the migration init container
// mounts as the OpenClaw source.
const MigrationSourceVolumeName = "openclaw-source"

// BuildMigrationInitContainer returns the init container that imports an
// OpenClaw instance into the hermes PVC. Returns nil when migration is not
// configured or already completed.
func BuildMigrationInitContainer(inst *hermesv1.HermesInstance) *corev1.Container {
	if inst.Spec.Migration.FromOpenClaw == nil {
		return nil
	}
	if inst.Status.Migration.Completed {
		return nil
	}
	fc := inst.Spec.Migration.FromOpenClaw

	image := fc.Image
	if image == "" {
		repo := inst.Spec.Image.Repository
		if repo == "" {
			repo = "ghcr.io/stubbi/hermes-agent"
		}
		tag := inst.Spec.Image.Tag
		if tag == "" {
			tag = "latest"
		}
		image = fmt.Sprintf("%s:%s", repo, tag)
	}

	var (
		args        []string
		mounts      []corev1.VolumeMount
		envFromList []corev1.EnvFromSource
		envList     []corev1.EnvVar
	)

	switch {
	case fc.Source.OpenClawInstanceRef != nil:
		args = []string{
			"-c",
			`set -euo pipefail
echo "Running hermes-agent importer from openclaw PVC mount" >&2
hermes-agent migrate from-openclaw --source /mnt/openclaw --dest /home/hermes/.hermes
`,
		}
		mounts = []corev1.VolumeMount{
			{Name: MigrationSourceVolumeName, MountPath: "/mnt/openclaw", ReadOnly: true},
			{Name: "data", MountPath: "/home/hermes/.hermes"},
		}

	case fc.Source.BackupRef != nil:
		s3 := fc.Source.BackupRef.S3
		args = []string{
			"-c",
			fmt.Sprintf(
				`set -euo pipefail
mkdir -p /mnt/openclaw
echo "Downloading OpenClaw snapshot %s/%s" >&2
aws --endpoint-url "https://%s" s3 cp "s3://%s/%s" - --no-progress | zstd -d | tar -xf - -C /mnt/openclaw
echo "Running hermes-agent importer against extracted snapshot" >&2
hermes-agent migrate from-openclaw --source /mnt/openclaw --dest /home/hermes/.hermes
`,
				s3.Bucket, s3.Key, s3.Endpoint, s3.Bucket, s3.Key,
			),
		}
		envFromList = []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: s3.CredentialsSecretRef.Name},
			},
		}}
		if s3.Region != "" {
			envList = append(envList, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: s3.Region})
		}
		mounts = []corev1.VolumeMount{
			{Name: MigrationSourceVolumeName, MountPath: "/mnt/openclaw"},
			{Name: "data", MountPath: "/home/hermes/.hermes"},
		}
	default:
		return nil
	}

	return &corev1.Container{
		Name:                     "init-migrate-from-openclaw",
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"/bin/sh"},
		Args:                     args,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env:                      envList,
		EnvFrom:                  envFromList,
		VolumeMounts:             mounts,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}
}
