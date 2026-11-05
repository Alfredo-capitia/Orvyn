//go:build unix

package executil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
