// Package instancelock provides a single-writer lock for Orvyn commands that
// mutate the stack.
//
// The lock is advisory and held for the lifetime of the process: the kernel
// releases it when the file descriptor is closed or the process exits, so a
// crash never leaves a lock that blocks the next run. The lock file itself is
// left in place on release; deleting it while another process is opening it
// would reintroduce a race.
package instancelock

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrLocked means another process holds the instance lock.
var ErrLocked = errors.New("another orvyn instance is already running")

// Lock is a held instance lock. Release must be called (usually with defer).
type Lock struct {
	f *os.File
}

// Acquire takes an exclusive, non-blocking lock on path.
//
// It fails with ErrLocked when another process already holds it. The error
// message includes the owner recorded in the file so an operator can find the
// process instead of guessing.
func Acquire(path string) (*Lock, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("instance lock: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("instance lock: create directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("instance lock: open %s: %w", path, err)
	}

	if err := tryLock(f); err != nil {
		owner := readOwner(f)
		f.Close()
		if errors.Is(err, errBusy) {
			if owner != "" {
				return nil, fmt.Errorf("%w (%s; lock: %s)", ErrLocked, owner, path)
			}
			return nil, fmt.Errorf("%w (lock: %s)", ErrLocked, path)
		}
		return nil, fmt.Errorf("instance lock: %s: %w", path, err)
	}

	// Recording the owner is best effort. It never affects the lock itself.
	recordOwner(f)
	return &Lock{f: f}, nil
}

// Release unlocks and closes the lock. It is safe to call more than once.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	unlockErr := unlock(l.f)
	closeErr := l.f.Close()
	l.f = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

// Path returns the lock file used by a state directory.
func Path(stateDir string) string {
	return filepath.Join(stateDir, "orvyn.lock")
}

func recordOwner(f *os.File) {
	owner := fmt.Sprintf("pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := f.Truncate(0); err != nil {
		return
	}
	if _, err := f.WriteAt([]byte(owner), 0); err != nil {
		return
	}
	_ = f.Sync()
}

func readOwner(f *os.File) string {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(f, 512))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
