package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSafeJoinTraversal(t *testing.T) {
	base := t.TempDir()
	_, err := SafeJoin(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestSafeJoinNormal(t *testing.T) {
	base := t.TempDir()
	p, err := SafeJoin(base, "config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, base) {
		t.Fatalf("path outside base: %s", p)
	}
}

func TestAtomicWrite(t *testing.T) {
	base := t.TempDir()
	p := filepath.Join(base, "test.txt")
	if err := AtomicWrite(p, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("wrong content: %s", string(b))
	}
}

func TestAtomicWriteSetsExactMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "file.yml")
	if err := AtomicWrite(p, []byte("data"), 0640); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0640 {
		t.Errorf("mode = %o, want 640", got)
	}
}

func TestAtomicWriteOverwritesExistingContent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file")
	if err := AtomicWrite(p, []byte("first"), 0600); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWrite(p, []byte("second"), 0600); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("content = %q, want second", data)
	}
	assertNoTempFiles(t, filepath.Dir(p))
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".orvyn-tmp-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestAtomicWriteCleansUpAfterRenameFailure(t *testing.T) {
	base := t.TempDir()
	// The destination is a directory, so the rename must fail.
	dest := filepath.Join(base, "destination")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := AtomicWrite(dest, []byte("data"), 0600); err == nil {
		t.Fatal("AtomicWrite succeeded when the destination was a directory")
	}
	assertNoTempFiles(t, base)
}

// Two writers sharing a fixed ".tmp" name would corrupt each other or fail on
// rename. The temporary names are unique, so the last completed write wins and
// no temporary file is left behind.
func TestAtomicWriteConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.json")

	const writers = 16
	payloads := make(map[string]struct{}, writers)
	for i := 0; i < writers; i++ {
		payloads[fmt.Sprintf(`{"writer":%d,"padding":"%s"}`, i, strings.Repeat("x", 256))] = struct{}{}
	}

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for body := range payloads {
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			errs <- AtomicWrite(path, []byte(body), 0600)
		}(body)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent AtomicWrite: %v", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if _, ok := payloads[string(data)]; !ok {
		t.Errorf("result is not one of the concurrent payloads: %q", data)
	}
	assertNoTempFiles(t, dir)
}

func TestAtomicWriteFailsWhenDirectoryIsAFile(t *testing.T) {
	base := t.TempDir()
	blocked := filepath.Join(base, "state")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := AtomicWrite(filepath.Join(blocked, "journal.json"), []byte("data"), 0600); err == nil {
		t.Fatal("AtomicWrite succeeded under a regular file")
	}
}
