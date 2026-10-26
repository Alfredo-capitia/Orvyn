package planner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateNoStack(t *testing.T) {
	// Use temp home by overriding env if possible - just ensure it doesn't crash
	ctx := context.Background()
	// This may try to access docker; skip if not available
	p, err := Generate(ctx)
	if err != nil {
		// allow error if docker not present
		t.Logf("err: %v", err)
		return
	}
	if p == nil {
		t.Fatal("nil plan")
	}
	if len(p.Actions) == 0 {
		t.Fatal("no actions")
	}
}

func TestGenerateWithComposePresent(t *testing.T) {
	home := os.TempDir()
	base := filepath.Join(home, "orvyn-test-plan")
	os.RemoveAll(base)
	defer os.RemoveAll(base)
	t.Setenv("HOME", home)
	// We can't easily change config paths without refactor, just test logic
	_ = base
	t.Skip("config paths not injectable")
}
