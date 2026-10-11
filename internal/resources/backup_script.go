package resources

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	hermesv1 "github.com/paperclipinc/hermes-operator/api/v1"
)

// The backup, prune and restore containers run shell scripts inside the
// snapshot-tool image (default restic/restic, which is Alpine based). The
// scripts therefore stick to what that image ships: busybox sh and tar plus
// restic. Compression is done by restic itself (repository format v2), so
// neither GNU tar nor zstd is required, and snapshot metadata travels as
// restic tags instead of a sidecar file inside the archive.

const (
	// BackupFormatVersion is recorded as a restic tag on every snapshot.
	// Version 1 was a zstd compressed tar (".tar.zst") with a meta.json
	// sidecar; version 2 is a plain tar of the data directory, compressed by
	// restic (".tar"), with metadata in restic tags.
	BackupFormatVersion = 2

	// backupDataDir is where the PVC is mounted in backup and restore containers.
	backupDataDir = "/home/hermes/.hermes"

	// backupTmpVolume is the emptyDir mounted at /tmp in backup and prune
	// containers. Their root filesystem stays read-only; restic and mktemp
	// need somewhere to write.
	backupTmpVolume = "backup-tmp"

	// DefaultResticPassword keys the restic repository when the credentials
	// Secret carries no RESTIC_PASSWORD. restic always requires a password;
	// without one the default builders could never open a repository. The
	// value is public, so confidentiality then rests on bucket access control.
	DefaultResticPassword = "hermes-operator-default-repository-password"
)

// resticPreamble is shared by every generated restic script.
func resticPreamble() string {
	return `set -eu
set -o pipefail
export TMPDIR="${TMPDIR:-/tmp}"
if [ -z "${RESTIC_PASSWORD:-}" ] && [ -z "${RESTIC_PASSWORD_FILE:-}" ]; then
  echo "WARNING: no RESTIC_PASSWORD in the credentials Secret; using the operator default repository password" >&2
  export RESTIC_PASSWORD=` + DefaultResticPassword + `
fi
`
}

// backupHost is the restic --host recorded on every snapshot of inst. It
// scopes retention to one instance, because all instances backing up to the
// same bucket share one restic repository.
func backupHost(inst *hermesv1.HermesInstance) string {
	return normalizedPathPrefix(inst) + inst.Namespace + "/" + inst.Name
}

// normalizedPathPrefix returns spec.backup.s3.pathPrefix with a trailing slash.
func normalizedPathPrefix(inst *hermesv1.HermesInstance) string {
	if inst.Spec.Backup.S3 == nil {
		return ""
	}
	p := inst.Spec.Backup.S3.PathPrefix
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// backupScript returns the script that snapshots backupDataDir into restic.
// keyAssign is a shell fragment that sets KEY (and may use TIMESTAMP).
func backupScript(inst *hermesv1.HermesInstance, keyAssign, kind string) string {
	return resticPreamble() + fmt.Sprintf(`TIMESTAMP=$(date -u +%%Y-%%m-%%dT%%H-%%M-%%SZ)
%s
if ! restic --no-cache cat config >/dev/null 2>&1; then
  echo "initializing restic repository $RESTIC_REPOSITORY" >&2
  restic --no-cache init
fi
tar -cf - -C %s . \
  | restic --no-cache backup --stdin --stdin-filename "$KEY" --host %q \
      --tag %q --tag %q --tag %q --tag "timestamp=$TIMESTAMP" \
  || { echo "BACKUP FAILED: $KEY" >&2; exit 1; }
echo "backup complete: $KEY" >&2
`,
		keyAssign,
		backupDataDir,
		backupHost(inst),
		kind,
		fmt.Sprintf("hermes-format=%d", BackupFormatVersion),
		"instance-uid="+string(inst.UID),
	)
}

// pruneScript returns the retention script for inst.
func pruneScript(inst *hermesv1.HermesInstance, historyLimit, failedHistoryLimit int32) string {
	host := backupHost(inst)
	return resticPreamble() + fmt.Sprintf(`if ! restic --no-cache cat config >/dev/null 2>&1; then
  echo "no restic repository at $RESTIC_REPOSITORY yet; nothing to prune" >&2
  exit 0
fi
restic --no-cache forget --host %q --group-by host --keep-last %d --prune --tag scheduled --tag onDelete --tag preUpdate
restic --no-cache forget --host %q --group-by host --keep-last %d --prune --tag failed
`, host, historyLimit, host, failedHistoryLimit)
}

// restoreScript returns the script that extracts snapshot key into backupDataDir.
// Format 2 snapshots (".tar") need only busybox tar; legacy format 1
// snapshots (".tar.zst") additionally need zstd in the image.
func restoreScript(key string) string {
	return resticPreamble() + fmt.Sprintf(`SNAPSHOT_KEY=%q
DEST=%s
if [ -n "$(ls -A "$DEST" 2>/dev/null)" ] && [ -z "${HERMES_RESTORE_FORCE:-}" ]; then
  echo "ERROR: restore destination $DEST is not empty; refusing to overwrite. Set HERMES_RESTORE_FORCE=1 to override." >&2
  exit 1
fi
case "$SNAPSHOT_KEY" in
  /*) SNAPSHOT_PATH="$SNAPSHOT_KEY" ;;
  *) SNAPSHOT_PATH="/$SNAPSHOT_KEY" ;;
esac
case "$SNAPSHOT_KEY" in
  *.zst)
    if ! command -v zstd >/dev/null 2>&1; then
      echo "ERROR: $SNAPSHOT_KEY is a format 1 (zstd) snapshot but zstd is not installed in this image; set spec.backup.image to an image with zstd" >&2
      exit 1
    fi
    restic --no-cache dump --path "$SNAPSHOT_PATH" latest "$SNAPSHOT_PATH" | zstd -d | tar -xf - -C "$DEST"
    ;;
  *)
    restic --no-cache dump --path "$SNAPSHOT_PATH" latest "$SNAPSHOT_PATH" | tar -xf - -C "$DEST"
    ;;
esac
echo "restore complete: $SNAPSHOT_KEY -> $DEST" >&2
`, key, backupDataDir)
}

// resticEnv returns the env shared by every restic container. The documented
// Secret keys S3_ACCESS_KEY_ID / S3_SECRET_ACCESS_KEY are mapped onto the
// AWS_* names restic reads. They are optional so a Secret that already uses
// the AWS_* names (consumed through envFrom) keeps working.
func resticEnv(s3 *hermesv1.BackupS3Spec, credSecretName string) []corev1.EnvVar {
	region := ""
	if s3 != nil {
		region = s3.Region
	}
	secretKey := func(env, key string) corev1.EnvVar {
		return corev1.EnvVar{Name: env, ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: credSecretName},
				Key:                  key,
				Optional:             Ptr(true),
			},
		}}
	}
	return []corev1.EnvVar{
		{Name: "RESTIC_REPOSITORY", Value: resticRepo(s3)},
		{Name: "AWS_DEFAULT_REGION", Value: region},
		{Name: "TMPDIR", Value: "/tmp"},
		secretKey("AWS_ACCESS_KEY_ID", "S3_ACCESS_KEY_ID"),
		secretKey("AWS_SECRET_ACCESS_KEY", "S3_SECRET_ACCESS_KEY"),
	}
}

// backupTmpVolumeMount mounts the scratch emptyDir at /tmp.
func backupTmpVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{Name: backupTmpVolume, MountPath: "/tmp"}
}

// backupTmpVolumeSource is the scratch emptyDir for backup and prune pods.
func backupTmpVolumeSource() corev1.Volume {
	return corev1.Volume{
		Name:         backupTmpVolume,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}
