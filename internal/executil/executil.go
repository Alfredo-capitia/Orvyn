package executil

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var ErrNotAllowed = errors.New("executable not in allowlist")

// Allowlist - keep minimal. Prefer Docker API over exec.
var allowlist = map[string]bool{
	"docker":         true,
	"docker-compose": true,
}

func isAllowed(name string) bool {
	return allowlist[name]
}

// RunWithTimeout runs allowed executables with args as separate values (no shell)
func RunWithTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	if !isAllowed(name) {
		return nil, fmt.Errorf("%w: %s", ErrNotAllowed, name)
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(c, name, args...)
	out, err := cmd.CombinedOutput()
	return out, err
}
