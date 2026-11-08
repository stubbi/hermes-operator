package resources

import (
	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
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

	args := []string{"-c", restoreScript(inst.Spec.RestoreFrom)}

	return &corev1.Container{
		Name:                     "init-restore",
		Image:                    image,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		Command:                  []string{"/bin/sh"},
		Args:                     args,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env:                      resticEnv(inst.Spec.Backup.S3, s3CredsSecretName(inst)),
		EnvFrom: []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: s3CredsSecretName(inst)},
			},
		}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: backupDataDir},
			// The pod-level "tmp" emptyDir: the root filesystem stays read-only.
			{Name: "tmp", MountPath: "/tmp"},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}
}
