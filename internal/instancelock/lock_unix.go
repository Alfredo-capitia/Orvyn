//go:build unix

package instancelock

import (
	"errors"
	"os"
	"syscall"
)

// errBusy is returned when the lock is held by another open file description.
var errBusy = errors.New("instance lock is held")

func tryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errBusy
	}
	return err
}

func unlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
