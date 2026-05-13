package resources

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hermesv1 "github.com/stubbi/hermes-operator/api/v1"
)

// BackupCronJobName returns the deterministic name for the periodic backup CronJob.
func BackupCronJobName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-backup-cron"
}

// BackupPruneCronJobName returns the deterministic name for the history-pruning CronJob.
func BackupPruneCronJobName(inst *hermesv1.HermesInstance) string {
	return inst.Name + "-backup-prune"
}

// BuildBackupCronJob returns the desired periodic backup CronJob. Caller is
// responsible for setting OwnerReferences and applying via CreateOrUpdate.
func BuildBackupCronJob(inst *hermesv1.HermesInstance) *batchv1.CronJob {
	s3 := inst.Spec.Backup.S3
	image := inst.Spec.Backup.Image
	if image == "" {
		image = ResticImage
	}

	labels := LabelsForInstance(inst)
	labels["hermes.agent/job-kind"] = "scheduled"

	historyLimit := int32(30)
	if inst.Spec.Backup.HistoryLimit != nil {
		historyLimit = *inst.Spec.Backup.HistoryLimit
	}
	failedHistoryLimit := int32(3)
	if inst.Spec.Backup.FailedHistoryLimit != nil {
		failedHistoryLimit = *inst.Spec.Backup.FailedHistoryLimit
	}

	region := ""
	pathPrefix := ""
	if s3 != nil {
		region = s3.Region
		pathPrefix = s3.PathPrefix
	}

	backoff := int32(3)
	ttl := int32(86400)
	gracePeriod := int64(30)

	// Shell command: compute timestamp at runtime; build the snapshot key under
	// `<pathPrefix><namespace>/<name>/<timestamp>.tar.zst`; archive and upload.
	args := []string{
		"-c",
		fmt.Sprintf(
			`set -euo pipefail
TIMESTAMP=$(date -u +%%Y-%%m-%%dT%%H-%%M-%%SZ)
KEY=%q
KEY="${KEY}${TIMESTAMP}.tar.zst"
META=$(mktemp)
jq -n --arg uid %q --arg ts "$TIMESTAMP" --arg fmt "1" \
    '{instance_uid:$uid, hermes_agent_version: env.HERMES_AGENT_VERSION // "", k8s_version: env.K8S_VERSION // "", timestamp:$ts, format_version:($fmt|tonumber)}' > "$META"
tar --use-compress-program="zstd -T0 -19" -cf - -C /home/hermes/.hermes . "$META" \
  | restic --repo "$RESTIC_REPOSITORY" --no-cache backup --stdin --stdin-filename "$KEY"
`,
			fmt.Sprintf("%s%s/%s/", pathPrefix, inst.Namespace, inst.Name),
			string(inst.UID),
		),
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyOnFailure,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: &gracePeriod,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: Ptr(true),
			RunAsUser:    Ptr(int64(1000)),
			RunAsGroup:   Ptr(int64(1000)),
			FSGroup:      Ptr(int64(1000)),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		// Co-locate on the same node as the StatefulSet pod so we can mount
		// the RWO PVC read-only.
		Affinity: &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name":     "hermes-agent",
							"app.kubernetes.io/instance": inst.Name,
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
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
			Env: []corev1.EnvVar{
				{Name: "RESTIC_REPOSITORY", Value: resticRepo(s3)},
				{Name: "AWS_DEFAULT_REGION", Value: region},
			},
			EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: s3CredsSecretName(inst)},
				},
			}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/home/hermes/.hermes", ReadOnly: true},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: Ptr(false),
				ReadOnlyRootFilesystem:   Ptr(true),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
		}},
		Volumes: []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: PVCName(inst),
					ReadOnly:  true,
				},
			},
		}},
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupCronJobName(inst),
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   inst.Spec.Backup.Schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &historyLimit,
			FailedJobsHistoryLimit:     &failedHistoryLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: batchv1.JobSpec{
					BackoffLimit:            &backoff,
					TTLSecondsAfterFinished: &ttl,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       podSpec,
					},
				},
			},
		},
	}
}

// BuildBackupPruneCronJob returns a daily CronJob that purges old snapshots.
//
// The prune logic:
//   - Lists `<prefix><ns>/<name>/*.tar.zst` sorted desc by lex timestamp.
//   - Keeps the newest `historyLimit`; deletes the rest.
//   - Lists `<prefix><ns>/<name>/failed/*.tar.zst` similarly with `failedHistoryLimit`.
//
// We run restic forget against the same repo using `--keep-last`. Restic's
// retention is content-aware so this is robust to clock skew.
func BuildBackupPruneCronJob(inst *hermesv1.HermesInstance) *batchv1.CronJob {
	s3 := inst.Spec.Backup.S3
	image := inst.Spec.Backup.Image
	if image == "" {
		image = ResticImage
	}
	labels := LabelsForInstance(inst)
	labels["hermes.agent/job-kind"] = "prune"

	historyLimit := int32(30)
	if inst.Spec.Backup.HistoryLimit != nil {
		historyLimit = *inst.Spec.Backup.HistoryLimit
	}
	failedHistoryLimit := int32(3)
	if inst.Spec.Backup.FailedHistoryLimit != nil {
		failedHistoryLimit = *inst.Spec.Backup.FailedHistoryLimit
	}

	successLim := int32(1)
	failLim := int32(3)

	region := ""
	if s3 != nil {
		region = s3.Region
	}
	backoff := int32(2)
	ttl := int32(86400)

	args := []string{
		"-c",
		fmt.Sprintf(
			`set -euo pipefail
restic --repo "$RESTIC_REPOSITORY" --no-cache forget --keep-last %d --prune --tag scheduled --tag onDelete --tag preUpdate
restic --repo "$RESTIC_REPOSITORY" --no-cache forget --keep-last %d --prune --tag failed
`,
			historyLimit, failedHistoryLimit,
		),
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyOnFailure,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: Ptr(int64(30)),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   Ptr(true),
			RunAsUser:      Ptr(int64(1000)),
			RunAsGroup:     Ptr(int64(1000)),
			FSGroup:        Ptr(int64(1000)),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Containers: []corev1.Container{{
			Name:                     "restic",
			Image:                    image,
			ImagePullPolicy:          corev1.PullIfNotPresent,
			Command:                  []string{"/bin/sh"},
			Args:                     args,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			Env: []corev1.EnvVar{
				{Name: "RESTIC_REPOSITORY", Value: resticRepo(s3)},
				{Name: "AWS_DEFAULT_REGION", Value: region},
			},
			EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: s3CredsSecretName(inst)},
				},
			}},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: Ptr(false),
				ReadOnlyRootFilesystem:   Ptr(true),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
		}},
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupPruneCronJobName(inst),
			Namespace: inst.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   "17 4 * * *",
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &successLim,
			FailedJobsHistoryLimit:     &failLim,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: batchv1.JobSpec{
					BackoffLimit:            &backoff,
					TTLSecondsAfterFinished: &ttl,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec:       podSpec,
					},
				},
			},
		},
	}
}
