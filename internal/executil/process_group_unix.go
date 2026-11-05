//go:build unix

package executil

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// processGroupWaitDelay bounds how long Wait keeps reading pipes after the
// context is cancelled and the group has been signalled.
const processGroupWaitDelay = 2 * time.Second

// configureProcessGroup puts the child in its own process group and makes
// cancellation kill the whole group.
//
// exec.CommandContext only kills the direct child. A compose wrapper or a
// shell script can leave descendants running with the output pipe open, so
// CombinedOutput would block until they exit even though the timeout already
// fired. Signalling -pid reaches every process the child started in its group.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			// The group is already gone; there is nothing left to kill.
			return nil
		}
		return err
	}
	cmd.WaitDelay = processGroupWaitDelay
}
