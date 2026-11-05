//go:build !unix

package executil

import "os/exec"

// configureProcessGroup is a no-op on platforms without process groups. The
// context still kills the direct child.
func configureProcessGroup(*exec.Cmd) {}
