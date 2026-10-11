package secretsbroker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Subcommand is the first CLI argument that routes the operator binary into
// the secrets broker helper instead of the controller manager.
const Subcommand = "secrets-broker"

// Main runs the helper. args excludes the program name and the Subcommand
// itself. It returns the process exit code; on a successful exec it never
// returns.
func Main(args []string, stderr io.Writer) int {
	logf := func(format string, a ...any) { _, _ = fmt.Fprintf(stderr, format+"\n", a...) }
	if len(args) == 0 {
		logf("usage: %s install <dest> | exec -- <entrypoint> [args...]", Subcommand)
		return 2
	}
	switch args[0] {
	case "install":
		if len(args) != 2 {
			logf("usage: %s install <dest>", Subcommand)
			return 2
		}
		if err := Install(args[1]); err != nil {
			logf("secrets broker: install: %v", err)
			return 1
		}
		return 0
	case "exec":
		rest := args[1:]
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			logf("usage: %s exec -- <entrypoint> [args...]", Subcommand)
			return 2
		}
		err := Exec(context.Background(), rest, os.Environ(), &Resolver{Log: logf}, syscall.Exec)
		logf("%v", err)
		return 1
	default:
		logf("secrets broker: unknown command %q", args[0])
		return 2
	}
}

// ExecFunc matches syscall.Exec so tests can intercept the final execve.
type ExecFunc func(argv0 string, argv []string, envv []string) error

// Exec resolves the secrets and replaces the current process with argv. It
// only returns on failure.
func Exec(ctx context.Context, argv []string, environ []string, r *Resolver, execve ExecFunc) error {
	env := envMap(environ)
	cfg, err := ParseConfig(env[ConfigEnv])
	if err != nil {
		return err
	}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	secrets, err := r.Resolve(ctx, cfg, lookup)
	if err != nil {
		return err
	}
	r.logf("secrets broker: resolved %d secret(s): %s", len(secrets), strings.Join(SortedNames(secrets), ", "))

	bin := argv[0]
	if !strings.Contains(bin, "/") {
		p, err := exec.LookPath(bin)
		if err != nil {
			return fmt.Errorf("secrets broker: entrypoint %q not found: %w", bin, err)
		}
		bin = p
	}
	if err := execve(bin, argv, MergeEnv(environ, secrets)); err != nil {
		return fmt.Errorf("secrets broker: exec %s: %w", bin, err)
	}
	return nil
}

func envMap(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}

// Install copies the running executable to dest (mode 0755) so the agent
// container, which uses a different image, can run it from a shared volume.
func Install(dest string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return copyFile(self, dest)
}

func copyFile(src, dest string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}
