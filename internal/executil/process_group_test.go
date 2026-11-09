//go:build unix

package executil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeDockerForProcessGroup installs a docker script in PATH and returns the
// path of a file the script can use to record descendants.
func fakeDockerForProcessGroup(t *testing.T, body string) string {
	t.Helper()

	binDir := t.TempDir()
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	stateFile := filepath.Join(t.TempDir(), "state")
	t.Setenv("FAKE_DOCKER_STATE", stateFile)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return stateFile
}

// A timed-out command must not wait for descendants that inherited its stdout
// pipe. exec.CommandContext only kills the direct child; the background sleep
// below survives that and keeps the pipe open for five seconds.
func TestRunWithTimeoutKillsTheWholeProcessGroup(t *testing.T) {
	stateFile := fakeDockerForProcessGroup(t, "sleep 5 &\nprintf '%s\\n' \"$!\" > \"$FAKE_DOCKER_STATE\"\nwait\n")

	start := time.Now()
	out, err := RunWithTimeout(context.Background(), 100*time.Millisecond, "docker", "compose", "up", "-d")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("command completed despite the timeout: %q", out)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("timeout took %s; a descendant kept the output pipe open (timeout was 100ms, sleep is 5s)", elapsed)
	}

	data, readErr := os.ReadFile(stateFile)
	if readErr != nil {
		t.Fatalf("read descendant pid: %v", readErr)
	}
	if pid := strings.TrimSpace(string(data)); pid == "" || pid == "0" {
		t.Fatalf("descendant pid = %q", pid)
	}
}

func TestRunWithTimeoutStillReturnsOutput(t *testing.T) {
	fakeDockerForProcessGroup(t, "printf 'compose-output\\n'\n")

	out, err := RunWithTimeout(context.Background(), 5*time.Second, "docker", "version")
	if err != nil {
		t.Fatalf("RunWithTimeout: %v", err)
	}
	if string(out) != "compose-output\n" {
		t.Errorf("output = %q, want compose-output", out)
	}
}

// The cancel hook can run before the command was started. In that case there
// is no process to signal, and the hook must be a no-op.
func TestProcessGroupCancelWithoutAProcess(t *testing.T) {
	old := killProcessGroup
	t.Cleanup(func() { killProcessGroup = old })

	called := false
	killProcessGroup = func(int) error {
		called = true
		return nil
	}

	cmd := exec.Command("true")
	configureProcessGroup(cmd)
	if cmd.Cancel == nil {
		t.Fatal("configureProcessGroup did not install a cancel hook")
	}
	if err := cmd.Cancel(); err != nil {
		t.Fatalf("Cancel without a process: %v", err)
	}
	if called {
		t.Error("Cancel tried to signal a group without a process")
	}
}

// The group may disappear between the context deadline and the signal. ESRCH
// means the desired state is already reached, so Cancel must not report it.
func TestProcessGroupCancelToleratesAGoneGroup(t *testing.T) {
	old := killProcessGroup
	t.Cleanup(func() { killProcessGroup = old })

	killProcessGroup = func(pid int) error {
		if pid != 12345 {
			t.Errorf("killProcessGroup(pid = %d), want 12345", pid)
		}
		return syscall.ESRCH
	}

	cmd := exec.Command("true")
	configureProcessGroup(cmd)
	cmd.Process = &os.Process{Pid: 12345}
	if err := cmd.Cancel(); err != nil {
		t.Fatalf("Cancel with an already-gone group: %v", err)
	}
}

// Other errors must reach the caller instead of being swallowed.
func TestProcessGroupCancelReturnsOtherErrors(t *testing.T) {
	old := killProcessGroup
	t.Cleanup(func() { killProcessGroup = old })

	killProcessGroup = func(int) error { return syscall.EPERM }

	cmd := exec.Command("true")
	configureProcessGroup(cmd)
	cmd.Process = &os.Process{Pid: 12345}
	if err := cmd.Cancel(); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("Cancel error = %v, want EPERM", err)
	}
}
