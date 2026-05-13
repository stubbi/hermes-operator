package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BuildRestoreInitContainer returns the init container that restores a snapshot
// into the PVC. Returns nil when no restore is requested or one already finished.
func BuildRestoreInitContainer(inst *hermesv1.HermesInstance) *corev1.Container {
	if inst.Spec.RestoreFrom == "" {
		return nil
	}
	if inst.Status.RestoredFrom == inst.Spec.RestoreFrom {
		return nil
	}
	if inst.Spec.Backup.S3 == nil {
		return nil
	}

	image := inst.Spec.Backup.Image
	if image == "" {
		image = ResticImage
	}
	region := inst.Spec.Backup.S3.Region

	args := []string{
		"-c",
		fmt.Sprintf(
			`set -euo pipefail
SNAPSHOT_KEY=%q
DEST=/home/hermes/.hermes
if [ -n "$(ls -A "$DEST" 2>/dev/null)" ] && [ -z "${HERMES_RESTORE_FORCE:-}" ]; then
  echo "ERROR: restore destination $DEST is not empty; refusing to overwrite. Set HERMES_RESTORE_FORCE=1 to override." >&2
  exit 1
fi
restic --repo "$RESTIC_REPOSITORY" --no-cache dump latest "$SNAPSHOT_KEY" \
  | zstd -d \
  | tar -xf - -C "$DEST"
echo "restore complete: $SNAPSHOT_KEY -> $DEST" >&2
`,
			inst.Spec.RestoreFrom,
		),
	}

	return &corev1.Container{
		Name:                     "init-restore",
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"/bin/sh"},
		Args:                     args,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env: []corev1.EnvVar{
			{Name: "RESTIC_REPOSITORY", Value: resticRepo(inst.Spec.Backup.S3)},
			{Name: "AWS_DEFAULT_REGION", Value: region},
		},
		EnvFrom: []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: s3CredsSecretName(inst)},
			},
		}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/home/hermes/.hermes"},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}
}
