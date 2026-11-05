package instancelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireRejectsSecondHolder(t *testing.T) {
	path := Path(t.TempDir())

	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Release()

	if _, err := Acquire(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second Acquire error = %v, want ErrLocked", err)
	}
}

func TestBusyErrorNamesTheHolder(t *testing.T) {
	path := Path(t.TempDir())

	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Release()

	_, err = Acquire(path)
	if err == nil {
		t.Fatal("second Acquire succeeded")
	}
	wantPID := fmt.Sprintf("pid=%d", os.Getpid())
	if !strings.Contains(err.Error(), wantPID) {
		t.Errorf("error %q does not name the holder (%s)", err, wantPID)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the lock file", err)
	}
}

func TestReleaseAllowsTheNextOwner(t *testing.T) {
	path := Path(t.TempDir())

	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	// Release must be idempotent: a double defer must not unlock an unrelated
	// descriptor.
	if err := first.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}

	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	defer second.Release()
}

func TestLockFileIsPrivate(t *testing.T) {
	path := Path(t.TempDir())

	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lock.Release()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("lock mode = %o, want 600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if !strings.Contains(string(data), "pid=") {
		t.Errorf("lock file does not record the owner: %q", data)
	}
}

func TestDifferentPathsDoNotConflict(t *testing.T) {
	base := t.TempDir()
	first, err := Acquire(Path(filepath.Join(base, "one")))
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Release()

	second, err := Acquire(Path(filepath.Join(base, "two")))
	if err != nil {
		t.Fatalf("second path was blocked: %v", err)
	}
	defer second.Release()
}

func TestAcquireRejectsEmptyPath(t *testing.T) {
	if _, err := Acquire(""); err == nil {
		t.Fatal("Acquire accepted an empty path")
	}
}

func TestAcquireReportsUnusablePath(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "state")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := Acquire(filepath.Join(blocked, "orvyn.lock")); err == nil {
		t.Fatal("Acquire succeeded under a regular file")
	}
}

func TestNilLockReleaseIsSafe(t *testing.T) {
	var lock *Lock
	if err := lock.Release(); err != nil {
		t.Errorf("nil Release: %v", err)
	}
}
