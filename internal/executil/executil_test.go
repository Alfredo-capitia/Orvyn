package executil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDocker puts an executable named "docker" first in PATH. The allowlist
// matches on the executable name, so this exercises the success and failure
// paths without touching a real daemon or installing anything.
func fakeDocker(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunWithTimeoutRunsAllowedExecutable(t *testing.T) {
	fakeDocker(t, "printf 'compose version v2'\n")

	out, err := RunWithTimeout(context.Background(), 5*time.Second, "docker", "compose", "version")
	if err != nil {
		t.Fatalf("RunWithTimeout: %v", err)
	}
	if got, want := string(out), "compose version v2"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunWithTimeoutReturnsExitError(t *testing.T) {
	fakeDocker(t, "echo boom >&2\nexit 3\n")

	_, err := RunWithTimeout(context.Background(), 5*time.Second, "docker", "ps")
	if err == nil {
		t.Fatal("RunWithTimeout returned nil for a failing command")
	}

	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("error = %v, want *exec.ExitError", err)
	}
	if exit.ExitCode() != 3 {
		t.Errorf("exit code = %d, want 3", exit.ExitCode())
	}
}

func TestRunWithTimeoutKillsSlowCommand(t *testing.T) {
	// exec replaces the shell, so the timed-out process is the one holding the
	// output pipe and CommandContext can reap it.
	fakeDocker(t, "exec sleep 5\n")

	start := time.Now()
	_, err := RunWithTimeout(context.Background(), 100*time.Millisecond, "docker", "version")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("RunWithTimeout returned nil for a command that exceeded the timeout")
	}
	if elapsed > 3*time.Second {
		t.Errorf("timeout took %s; the child process was not killed promptly", elapsed)
	}
}

func TestRunWithTimeoutHonoursCancelledContext(t *testing.T) {
	fakeDocker(t, "sleep 5\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunWithTimeout(ctx, time.Minute, "docker", "version")
	if err == nil {
		t.Fatal("RunWithTimeout returned nil with a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

// Arguments are passed as separate values: a shell metacharacter must reach the
// process literally instead of being interpreted.
func TestRunWithTimeoutDoesNotUseAShell(t *testing.T) {
	fakeDocker(t, "for arg in \"$@\"; do printf '%s\\n' \"$arg\"; done\n")

	marker := filepath.Join(t.TempDir(), "injected")
	payload := "x; touch " + marker
	out, err := RunWithTimeout(context.Background(), 5*time.Second, "docker", payload)
	if err != nil {
		t.Fatalf("RunWithTimeout: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != payload {
		t.Errorf("argument = %q, want it passed literally as %q", got, payload)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("shell metacharacters were interpreted by the child process")
	}
}

func TestRunInteractiveRunsAllowedExecutable(t *testing.T) {
	fakeDocker(t, "exit 0\n")

	if err := RunInteractive(context.Background(), "docker", "ps"); err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
}

func TestRunInteractiveReturnsExitError(t *testing.T) {
	fakeDocker(t, "exit 7\n")

	err := RunInteractive(context.Background(), "docker", "ps")
	if err == nil {
		t.Fatal("RunInteractive returned nil for a failing command")
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("error = %v, want exit code 7", err)
	}
}

func TestRunInteractiveRejectsUnknownExecutable(t *testing.T) {
	err := RunInteractive(context.Background(), "sh", "-c", "true")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("RunInteractive(sh) = %v, want ErrNotAllowed", err)
	}
}

func TestRunWithTimeoutRejectsUnknownExecutable(t *testing.T) {
	_, err := RunWithTimeout(context.Background(), 0, "sh", "-c", "true")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("RunWithTimeout(sh) = %v, want ErrNotAllowed", err)
	}
}
