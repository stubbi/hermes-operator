package resources

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

// ResticImage is the pinned default snapshot-tool image. Mirrors
// internal/controller.ResticImage (duplicated to keep this package import-cycle free).
const ResticImage = "restic/restic:0.16.4"

// BackupJobOpts captures the inputs the controller passes to the builder.
type BackupJobOpts struct {
	Name        string // Deterministic Job name (e.g. "<inst>-backup-final")
	SnapshotKey string // Full S3 key the snapshot will be written to
	Kind        string // "onDelete" | "preUpdate" | "scheduled": recorded as a label
}

// BuildBackupOneShotJob returns a Job that snapshots the instance PVC to S3.
// S3 access keys arrive via EnvFrom.SecretRef so they never appear in the PodSpec.
func BuildBackupOneShotJob(inst *hermesv1.HermesInstance, opts BackupJobOpts) *batchv1.Job {
	image := inst.Spec.Backup.Image
	if image == "" {
		image = ResticImage
	}

	s3 := inst.Spec.Backup.S3
	credSecretName := ""
	if s3 != nil {
		credSecretName = s3.CredentialsSecretRef.Name
	}

	labels := LabelsForInstance(inst)
	labels["hermes.agent/job-kind"] = opts.Kind

	backoff := int32(3)
	ttl := int32(86400)

	args := []string{
		"-c",
		backupScript(inst, fmt.Sprintf("KEY=%q", opts.SnapshotKey), opts.Kind),
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyOnFailure,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: Ptr(int64(30)),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: Ptr(true),
						RunAsUser:    Ptr(int64(1000)),
						RunAsGroup:   Ptr(int64(1000)),
						FSGroup:      Ptr(int64(1000)),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Name:                     "restic",
						Image:                    image,
						ImagePullPolicy:          corev1.PullIfNotPresent,
						Command:                  []string{"/bin/sh"},
						Args:                     args,
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						Env:                      resticEnv(s3, credSecretName),
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: credSecretName},
							},
						}},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: backupDataDir},
							backupTmpVolumeMount(),
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: Ptr(false),
							ReadOnlyRootFilesystem:   Ptr(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: PVCName(inst),
								},
							},
						},
						backupTmpVolumeSource(),
					},
				},
			},
		},
	}
}

func resticRepo(s3 *hermesv1.BackupS3Spec) string {
	if s3 == nil {
		return ""
	}
	return fmt.Sprintf("s3:%s/%s", s3.Endpoint, s3.Bucket)
}

func s3CredsSecretName(inst *hermesv1.HermesInstance) string {
	if inst.Spec.Backup.S3 == nil {
		return ""
	}
	return inst.Spec.Backup.S3.CredentialsSecretRef.Name
}
