package fsutil

import (
	"os"
	"path/filepath"
	"strings"
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
