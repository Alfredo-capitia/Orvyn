package executil

import (
	"context"
	"errors"
	"testing"
)

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
