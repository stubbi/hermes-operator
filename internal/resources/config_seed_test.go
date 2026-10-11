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

func TestBuildStatefulSet_ConfigSeedInitContainer(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Image.Repository = "ghcr.io/example/hermes-agent"
	inst.Spec.Image.Tag = "v1.2.3"
	inst.Spec.InitContainers = []corev1.Container{{Name: "user-init", Image: "alpine"}}
	restore := corev1.Container{Name: "init-restore", Image: "restic"}

	sts := BuildStatefulSet(inst, []corev1.Container{restore})
	inits := sts.Spec.Template.Spec.InitContainers
	names := make([]string, 0, len(inits))
	for _, c := range inits {
		names = append(names, c.Name)
	}
	// Order matters: restore first (it rewrites the volume), then config-seed,
	// then user init containers (which should see the final config.yaml).
	assert.Equal(t, []string{"init-restore", ConfigSeedContainerName, "user-init"}, names)

	seed := inits[1]
	assert.Equal(t, "ghcr.io/example/hermes-agent:v1.2.3", seed.Image, "reuses the agent image so the migration code matches the agent")
	require.Len(t, seed.Command, 3)
	assert.Equal(t, []string{"/bin/sh", "-c"}, seed.Command[:2], "must bypass the s6 /init entrypoint")
	assert.Equal(t, corev1.TerminationMessageFallbackToLogsOnError, seed.TerminationMessagePolicy)
	assert.Equal(t, sts.Spec.Template.Spec.Containers[0].Resources, seed.Resources)

	mounts := map[string]corev1.VolumeMount{}
	for _, m := range seed.VolumeMounts {
		mounts[m.Name] = m
	}
	assert.Equal(t, "/opt/data", mounts["data"].MountPath)
	assert.False(t, mounts["data"].ReadOnly)
	assert.Equal(t, ConfigStagingMountPath, mounts["config"].MountPath)
	assert.True(t, mounts["config"].ReadOnly)
	assert.Empty(t, mounts["config"].SubPath, "staging is a directory mount, not a subPath")
}

func TestBuildConfigSeedContainer_ForwardsSkipMigrationEnv(t *testing.T) {
	t.Parallel()
	inst := minimalInstance()
	inst.Spec.Env = []corev1.EnvVar{
		{Name: "OPENAI_API_KEY", Value: "should-not-leak-into-init"},
		{Name: skipConfigMigrationEnv, Value: "1"},
	}
	c := BuildConfigSeedContainer(inst, nil)
	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
	}
	assert.Equal(t, "1", env[skipConfigMigrationEnv])
	assert.Equal(t, "/opt/data", env["HERMES_HOME"])
	assert.NotContains(t, env, "OPENAI_API_KEY")
}

// runSeedScript executes configSeedScript with its absolute paths rewritten
// into a temp dir. fakeMigrate, when non-empty, is installed as the image's
// docker_config_migrate.py and run by a fake python (plain sh).
func runSeedScript(t *testing.T, fakeMigrate string) (home string, out string, err error) {
	t.Helper()
	if _, lookErr := exec.LookPath("sh"); lookErr != nil {
		t.Skip("sh not available")
	}
	root := t.TempDir()
	home = filepath.Join(root, "data")
	staging := filepath.Join(root, "staging")
	install := filepath.Join(root, "hermes")
	require.NoError(t, os.MkdirAll(home, 0o755))
	require.NoError(t, os.MkdirAll(staging, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "config.yaml"), []byte("model: declared\n"), 0o644))
	// A stale, agent-modified config must be replaced by the declared one.
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.yaml"), []byte("model: stale\n"), 0o644))

	if fakeMigrate != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(install, ".venv", "bin"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(install, "scripts"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(install, ".venv", "bin", "python"), []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(install, "scripts", "docker_config_migrate.py"), []byte(fakeMigrate), 0o644))
	}

	script := strings.ReplaceAll(configSeedScript, ConfigStagingMountPath, staging)
	script = strings.ReplaceAll(script, "/opt/hermes", install)
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "HERMES_HOME="+home, "HERMES_UID=1000", "HERMES_GID=1000")
	b, err := cmd.CombinedOutput()
	return home, string(b), err
}

func TestConfigSeedScript_CopiesWithoutMigrator(t *testing.T) {
	t.Parallel()
	home, out, err := runSeedScript(t, "")
	require.NoError(t, err, out)
	got, readErr := os.ReadFile(filepath.Join(home, "config.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "model: declared\n", string(got), "ConfigMap stays authoritative on every start")
	assert.Contains(t, out, "skipping init time config migration")
	_, statErr := os.Stat(filepath.Join(home, "config.yaml.operator-tmp"))
	assert.True(t, os.IsNotExist(statErr), "temp file must be renamed away")
}

func TestConfigSeedScript_MigrationSucceeds(t *testing.T) {
	t.Parallel()
	// The fake migration rewrites the file in place, proving it is writable.
	home, out, err := runSeedScript(t, "echo '_config_version: 42' >> \"$HERMES_HOME/config.yaml\"\n")
	require.NoError(t, err, out)
	got, readErr := os.ReadFile(filepath.Join(home, "config.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "model: declared\n_config_version: 42\n", string(got))
	assert.Contains(t, out, "config schema is current")
}

func TestConfigSeedScript_MigrationFailureFailsInit(t *testing.T) {
	t.Parallel()
	_, out, err := runSeedScript(t, "echo '[config-migrate] ERROR: boom' >&2\nexit 1\n")
	require.Error(t, err, "a failed migration must fail the init container instead of warning and continuing")
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, out, "[config-migrate] ERROR: boom")
	assert.Contains(t, out, "config schema migration failed")
}
