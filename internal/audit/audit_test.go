package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditLog(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.jsonl")
	l := New(p)
	err := l.Log(Event{Operation: "test.op", Result: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "test.op") {
		t.Fatalf("missing event: %s", string(b))
	}
}
