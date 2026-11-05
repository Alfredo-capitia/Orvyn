package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/instancelock"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// lockTestEnv is a minimal isolated host for the lock tests. It intentionally
// does not depend on helpers from other test files so it can live on its own
// branch: a fake HOME, a compose file, a fake docker that records its
// arguments and a fake daemon that reports an empty container list.
type lockTestEnv struct {
	home      string
	dockerLog string
}

func newLockTestEnv(t *testing.T) *lockTestEnv {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USER", "audit-test")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")

	cfg := config.Default()
	if err := config.EnsureDirs(cfg); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	compose := "name: \"orvyn-locktest\"\n\nservices: {}\n"
	if err := os.WriteFile(cfg.StackCompose(), []byte(compose), 0o600); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	env := &lockTestEnv{home: home, dockerLog: filepath.Join(home, "docker.log")}
	binDir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$LOCK_TEST_DOCKER_LOG\"\nprintf 'fake-output\\n'\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("LOCK_TEST_DOCKER_LOG", env.dockerLog)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())

	return env
}

func (e *lockTestEnv) dockerCalls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e.dockerLog)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read docker log: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func lockTestAuditEvents(t *testing.T) []audit.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.Default().StateDir, "audit.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read audit log: %v", err)
	}
	var events []audit.Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev audit.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("decode audit event %q: %v", line, err)
		}
		events = append(events, ev)
	}
	return events
}

func lockTestRun(t *testing.T, cmd *cobra.Command, stdin string, args ...string) (string, error) {
	t.Helper()

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for i := 0; i+1 < len(args); i += 2 {
		if err := cmd.Flags().Set(args[i], args[i+1]); err != nil {
			t.Fatalf("set --%s: %v", args[i], err)
		}
	}

	in, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create stdin: %v", err)
	}
	if _, err := in.WriteString(stdin); err != nil {
		t.Fatalf("seed stdin: %v", err)
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind stdin: %v", err)
	}
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout: %v", err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	defer func() {
		os.Stdin, os.Stdout = oldIn, oldOut
		in.Close()
		out.Close()
	}()

	runErr := cmd.RunE(cmd, nil)
	if err := out.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data), runErr
}

func lockTestAcquire(t *testing.T) *instancelock.Lock {
	t.Helper()
	lock, err := instancelock.Acquire(instancelock.Path(config.Default().StateDir))
	if err != nil {
		t.Fatalf("pre-acquire instance lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

func TestMutatingCommandsRefuseToRunWhileLocked(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		flag string
		op   string
	}{
		{"apply", applyCmd, "", "apply"},
		{"down", downCmd, "yes", "down"},
		{"rm", rmCmd, "yes", "rm"},
		{"uninstall", uninstallCmd, "force", "uninstall"},
		{"prune", pruneCmd, "yes", "prune"},
		{"upgrade", upgradeCmd, "yes", "upgrade"},
		{"restore", restoreCmd, "yes", "restore"},
		{"start", startCmd, "", "start"},
		{"stop", stopCmd, "", "stop"},
		{"restart", restartCmd, "", "restart"},
		{"up", upCmd, "", "up"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newLockTestEnv(t)
			lockTestAcquire(t)
			path := instancelock.Path(config.Default().StateDir)

			var args []string
			if tc.flag != "" {
				args = []string{tc.flag, "true"}
			}
			_, err := lockTestRun(t, tc.cmd, "y\n", args...)
			if !errors.Is(err, instancelock.ErrLocked) {
				t.Fatalf("%s error = %v, want ErrLocked", tc.name, err)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("%s error does not name the lock file: %v", tc.name, err)
			}

			if calls := env.dockerCalls(t); len(calls) != 0 {
				t.Errorf("%s executed docker despite the lock: %v", tc.name, calls)
			}

			events := lockTestAuditEvents(t)
			if len(events) != 1 {
				t.Fatalf("%s wrote %d audit events, want 1: %+v", tc.name, len(events), events)
			}
			if events[0].Operation != tc.op || events[0].Result != "locked" {
				t.Errorf("%s audit event = %+v, want operation %q result locked", tc.name, events[0], tc.op)
			}
		})
	}
}

func TestLockIsReleasedAfterAnAbortedCommand(t *testing.T) {
	newLockTestEnv(t)

	out, err := lockTestRun(t, downCmd, "n\n")
	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if !strings.Contains(out, "Abort") {
		t.Fatalf("down did not abort:\n%s", out)
	}

	lock, err := instancelock.Acquire(instancelock.Path(config.Default().StateDir))
	if err != nil {
		t.Fatalf("lock was not released after abort: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestLockIsReleasedAfterASuccessfulCommand(t *testing.T) {
	env := newLockTestEnv(t)

	if _, err := lockTestRun(t, downCmd, "", "yes", "true"); err != nil {
		t.Fatalf("down: %v", err)
	}
	if calls := env.dockerCalls(t); len(calls) == 0 {
		t.Fatal("down did not run docker")
	}

	lock, err := instancelock.Acquire(instancelock.Path(config.Default().StateDir))
	if err != nil {
		t.Fatalf("lock was not released after success: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// TestLockIsReleasedAfterEveryMutatingCommand runs each command that now takes
// the lock and proves the lock is free afterwards, whatever the command result
// was. It complements TestMutatingCommandsRefuseToRunWhileLocked.
func TestLockIsReleasedAfterEveryMutatingCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"prune", pruneCmd, []string{"yes", "true"}},
		{"upgrade", upgradeCmd, []string{"yes", "true"}},
		{"restore", restoreCmd, []string{"yes", "true"}},
		{"start", startCmd, nil},
		{"stop", stopCmd, nil},
		{"restart", restartCmd, nil},
		{"up", upCmd, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newLockTestEnv(t)

			// The command may fail for environment reasons (for example restore
			// without a backup). What matters here is that it releases the lock
			// on every return path.
			_, _ = lockTestRun(t, tc.cmd, "y\n", tc.args...)

			lock, err := instancelock.Acquire(instancelock.Path(config.Default().StateDir))
			if err != nil {
				t.Fatalf("%s left the instance lock held: %v", tc.name, err)
			}
			if err := lock.Release(); err != nil {
				t.Fatalf("%s: release: %v", tc.name, err)
			}
		})
	}
}
