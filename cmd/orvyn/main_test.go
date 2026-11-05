package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildOrvyn compiles the command into a temporary directory so the smoke test
// exercises the same entry point operators run.
func buildOrvyn(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the binary; skipped in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "orvyn")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// runOrvyn runs the binary and returns stdout, stderr and the exit code.
func runOrvyn(t *testing.T, bin string, args ...string) (string, string, int) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func TestMainVersion(t *testing.T) {
	bin := buildOrvyn(t)

	stdout, stderr, code := runOrvyn(t, bin, "version")
	if code != 0 {
		t.Fatalf("version exited %d, want 0 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "orvyn v") {
		t.Errorf("version stdout = %q, want it to contain %q", stdout, "orvyn v")
	}
	if stderr != "" {
		t.Errorf("version stderr = %q, want empty", stderr)
	}
}

func TestMainUnknownCommandFails(t *testing.T) {
	bin := buildOrvyn(t)

	stdout, stderr, code := runOrvyn(t, bin, "definitely-not-a-command")
	if code != 1 {
		t.Fatalf("unknown command exited %d, want 1 (stdout: %q, stderr: %q)", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("unknown command stderr = %q, want it to contain %q", stderr, "Error:")
	}
}

// captureOutput redirects the process streams while fn runs, so the tests can
// call run directly instead of spawning the binary.
func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout, os.Stderr = outW, errW
	fn()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	if err := outW.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	if err := errW.Close(); err != nil {
		t.Fatalf("close stderr: %v", err)
	}
	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errOut, err := io.ReadAll(errR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(out), string(errOut)
}

// setArgs replaces os.Args for the duration of a test.
func setArgs(t *testing.T, args ...string) {
	t.Helper()
	old := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = old })
}

func TestRunVersion(t *testing.T) {
	setArgs(t, "orvyn", "version")

	var code int
	stdout, stderr := captureOutput(t, func() { code = run() })

	if code != 0 {
		t.Fatalf("run() = %d, want 0 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "orvyn v") {
		t.Errorf("stdout = %q, want it to contain %q", stdout, "orvyn v")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	setArgs(t, "orvyn", "definitely-not-a-command")

	var code int
	stdout, stderr := captureOutput(t, func() { code = run() })

	if code != 1 {
		t.Fatalf("run() = %d, want 1 (stdout: %q)", code, stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "Error:")
	}
}
