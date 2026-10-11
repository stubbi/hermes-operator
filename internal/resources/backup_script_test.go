package resources

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

// Regression coverage for #165: the generated backup, prune and restore
// containers must run in the default Alpine based restic image under a
// read-only root filesystem.

type resticContainer struct {
	name      string
	container corev1.Container
	volumes   []corev1.Volume
}

func allResticContainers(t *testing.T) []resticContainer {
	t.Helper()
	inst := backupInstance()
	cron := BuildBackupCronJob(inst).Spec.JobTemplate.Spec.Template.Spec
	prune := BuildBackupPruneCronJob(inst).Spec.JobTemplate.Spec.Template.Spec
	job := BuildBackupOneShotJob(inst, BackupJobOpts{
		Name: "demo-backup-final", SnapshotKey: "prod/agents/demo/2026-05-10T03-00-00Z.tar", Kind: "onDelete",
	}).Spec.Template.Spec

	rinst := restoreInstance()
	restore := BuildRestoreInitContainer(rinst)
	require.NotNil(t, restore)
	sts := BuildStatefulSet(rinst, []corev1.Container{*restore}).Spec.Template.Spec

	return []resticContainer{
		{"cron", cron.Containers[0], cron.Volumes},
		{"prune", prune.Containers[0], prune.Volumes},
		{"oneshot", job.Containers[0], job.Volumes},
		{"restore", *restore, sts.Volumes},
	}
}

func scriptOf(c corev1.Container) string {
	return c.Args[len(c.Args)-1]
}

func TestResticContainers_WritableTmpUnderReadOnlyRootFS(t *testing.T) {
	for _, rc := range allResticContainers(t) {
		t.Run(rc.name, func(t *testing.T) {
			sc := rc.container.SecurityContext
			require.NotNil(t, sc)
			require.NotNil(t, sc.ReadOnlyRootFilesystem)
			assert.True(t, *sc.ReadOnlyRootFilesystem, "root filesystem must stay read-only")

			var tmpVol string
			for _, vm := range rc.container.VolumeMounts {
				if vm.MountPath == "/tmp" {
					tmpVol = vm.Name
					assert.False(t, vm.ReadOnly)
				}
			}
			require.NotEmpty(t, tmpVol, "container needs a writable /tmp mount")
			found := false
			for _, v := range rc.volumes {
				if v.Name == tmpVol {
					found = true
					assert.NotNil(t, v.EmptyDir, "/tmp must be backed by an emptyDir")
				}
			}
			assert.True(t, found, "volume %q backing /tmp is missing from the pod", tmpVol)

			env := map[string]corev1.EnvVar{}
			for _, e := range rc.container.Env {
				env[e.Name] = e
			}
			assert.Equal(t, "/tmp", env["TMPDIR"].Value)
		})
	}
}

func TestResticContainers_MapDocumentedSecretKeysToAWSNames(t *testing.T) {
	for _, rc := range allResticContainers(t) {
		t.Run(rc.name, func(t *testing.T) {
			env := map[string]corev1.EnvVar{}
			for _, e := range rc.container.Env {
				env[e.Name] = e
			}
			for awsName, secretKey := range map[string]string{
				"AWS_ACCESS_KEY_ID":     "S3_ACCESS_KEY_ID",
				"AWS_SECRET_ACCESS_KEY": "S3_SECRET_ACCESS_KEY",
			} {
				e, ok := env[awsName]
				require.True(t, ok, "missing %s", awsName)
				require.NotNil(t, e.ValueFrom)
				require.NotNil(t, e.ValueFrom.SecretKeyRef)
				assert.Equal(t, "hermes-s3-creds", e.ValueFrom.SecretKeyRef.Name)
				assert.Equal(t, secretKey, e.ValueFrom.SecretKeyRef.Key)
				require.NotNil(t, e.ValueFrom.SecretKeyRef.Optional)
				assert.True(t, *e.ValueFrom.SecretKeyRef.Optional,
					"optional so Secrets that already carry AWS_* keys via envFrom keep working")
			}
			// envFrom stays so RESTIC_PASSWORD and other restic settings pass through.
			require.Len(t, rc.container.EnvFrom, 1)
			assert.Equal(t, "hermes-s3-creds", rc.container.EnvFrom[0].SecretRef.Name)
		})
	}
}

func TestResticScripts_OnlyUseToolsInTheDefaultImage(t *testing.T) {
	for _, rc := range allResticContainers(t) {
		t.Run(rc.name, func(t *testing.T) {
			s := scriptOf(rc.container)
			assert.NotContains(t, s, "--use-compress-program", "busybox tar has no --use-compress-program")
			assert.NotContains(t, s, "jq ", "metadata travels as restic tags, jq is not needed")
			assert.NotContains(t, s, "mktemp")
			assert.Contains(t, s, "set -o pipefail")
			assert.Contains(t, s, "RESTIC_PASSWORD")
			if rc.name != "restore" {
				assert.NotContains(t, s, "zstd", "restic compresses; zstd is not in the default image")
			}
		})
	}
}

func TestBackupScript_TagsHostAndInit(t *testing.T) {
	inst := backupInstance()
	cron := scriptOf(BuildBackupCronJob(inst).Spec.JobTemplate.Spec.Template.Spec.Containers[0])
	assert.Contains(t, cron, `KEY="prod/agents/demo/""${TIMESTAMP}.tar"`)
	assert.Contains(t, cron, `--tag "scheduled"`)
	assert.Contains(t, cron, `--host "prod/agents/demo"`)
	assert.Contains(t, cron, `--tag "hermes-format=2"`)
	assert.Contains(t, cron, `--tag "instance-uid=uid-1"`)
	assert.Contains(t, cron, "restic --no-cache init")

	job := scriptOf(BuildBackupOneShotJob(inst, BackupJobOpts{
		Name: "x", SnapshotKey: "prod/agents/demo/2026-05-10T03-00-00Z.tar", Kind: "preUpdate",
	}).Spec.Template.Spec.Containers[0])
	assert.Contains(t, job, `KEY="prod/agents/demo/2026-05-10T03-00-00Z.tar"`)
	assert.Contains(t, job, `--tag "preUpdate"`)
}

func TestBackupScript_PathPrefixWithoutTrailingSlash(t *testing.T) {
	inst := backupInstance()
	inst.Spec.Backup.S3.PathPrefix = "prod"
	cron := scriptOf(BuildBackupCronJob(inst).Spec.JobTemplate.Spec.Template.Spec.Containers[0])
	assert.Contains(t, cron, `KEY="prod/agents/demo/""${TIMESTAMP}.tar"`)
}

func TestPruneScript_ScopedToInstance(t *testing.T) {
	s := scriptOf(BuildBackupPruneCronJob(backupInstance()).Spec.JobTemplate.Spec.Template.Spec.Containers[0])
	assert.Contains(t, s, `forget --host "prod/agents/demo" --group-by host --keep-last 30 --prune --tag scheduled --tag onDelete --tag preUpdate`)
	assert.Contains(t, s, `forget --host "prod/agents/demo" --group-by host --keep-last 3 --prune --tag failed`)
}

func TestRestoreScript_LegacyZstdOnlyForZstKeys(t *testing.T) {
	s := restoreScript("prod/agents/demo/2026-05-10T03-00-00Z.tar")
	assert.Contains(t, s, `restic --no-cache dump --path "$SNAPSHOT_PATH" latest "$SNAPSHOT_PATH" | tar -xf - -C "$DEST"`)
	assert.Contains(t, s, "*.zst)")
	assert.Contains(t, s, "command -v zstd")
}

// TestResticScripts_ShellSyntax parses every generated script with sh -n
// (and busybox sh when available, which is what the default image runs).
func TestResticScripts_ShellSyntax(t *testing.T) {
	shells := []string{}
	if p, err := exec.LookPath("sh"); err == nil {
		shells = append(shells, p)
	}
	if p, err := exec.LookPath("busybox"); err == nil {
		shells = append(shells, p+" sh")
	}
	if len(shells) == 0 {
		t.Skip("no sh available")
	}
	for _, rc := range allResticContainers(t) {
		for _, sh := range shells {
			parts := strings.Fields(sh)
			args := make([]string, 0, len(parts)+2)
			args = append(args, parts[1:]...)
			args = append(args, "-n", "-c", scriptOf(rc.container))
			out, err := exec.Command(parts[0], args...).CombinedOutput()
			assert.NoError(t, err, "%s: %s rejected the script: %s", rc.name, sh, out)
		}
	}
}

// TestResticScripts_BackupRestoreRoundTrip runs the generated backup, prune and
// restore scripts against a local restic repository. It needs a restic binary
// on PATH and is skipped otherwise.
func TestResticScripts_BackupRestoreRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	root := t.TempDir()
	data := filepath.Join(root, "data")
	dest := filepath.Join(root, "dest")
	tmp := filepath.Join(root, "tmp")
	for _, d := range []string{filepath.Join(data, "skills"), dest, tmp} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(data, "config.yaml"), []byte("model: x\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(data, "skills", "a.md"), []byte("skill\n"), 0o600))

	run := func(script, dataDir string) string {
		script = strings.ReplaceAll(script, backupDataDir, dataDir)
		cmd := exec.Command("sh", "-c", script)
		cmd.Env = append(os.Environ(),
			"RESTIC_REPOSITORY="+filepath.Join(root, "repo"),
			"RESTIC_PASSWORD=",
			"TMPDIR="+tmp,
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "script failed:\n%s\n--- output ---\n%s", script, out)
		return string(out)
	}

	inst := backupInstance()
	key := "prod/agents/demo/2026-05-10T03-00-00Z.tar"
	out := run(scriptOf(BuildBackupOneShotJob(inst, BackupJobOpts{Name: "x", SnapshotKey: key, Kind: "onDelete"}).
		Spec.Template.Spec.Containers[0]), data)
	assert.Contains(t, out, "initializing restic repository")
	assert.Contains(t, out, "default repository password")
	run(scriptOf(BuildBackupCronJob(inst).Spec.JobTemplate.Spec.Template.Spec.Containers[0]), data)
	run(scriptOf(BuildBackupPruneCronJob(inst).Spec.JobTemplate.Spec.Template.Spec.Containers[0]), data)

	run(restoreScript(key), dest)
	got, err := os.ReadFile(filepath.Join(dest, "skills", "a.md"))
	require.NoError(t, err)
	assert.Equal(t, "skill\n", string(got))
	got, err = os.ReadFile(filepath.Join(dest, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "model: x\n", string(got))
}
