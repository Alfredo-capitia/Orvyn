package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	c := Default()
	if c.StackDir == "" || c.ConfigDir == "" || c.StateDir == "" || c.BackupDir == "" {
		t.Fatal("empty paths")
	}
	if filepath.Base(c.StackDir) != "stack" {
		t.Fatalf("unexpected stack dir: %s", c.StackDir)
	}
	if c.StackCompose() == "" {
		t.Fatal("empty compose path")
	}
}

func TestStackCompose(t *testing.T) {
	c := &Config{StackDir: "/tmp/orvyn-stack"}
	got := c.StackCompose()
	if got != filepath.Join("/tmp/orvyn-stack", "docker-compose.yml") {
		t.Fatalf("got %s", got)
	}
}

func TestEnsureDirsCreatesLayout(t *testing.T) {
	base := t.TempDir()
	c := &Config{
		StackDir:  filepath.Join(base, "stack"),
		BackupDir: filepath.Join(base, "backups"),
		ConfigDir: filepath.Join(base, "config"),
		StateDir:  filepath.Join(base, "state"),
	}

	if err := EnsureDirs(c); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, dir := range []string{c.StackDir, c.BackupDir, c.ConfigDir, c.StateDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// The state directory holds the apply journal and the audit log; it must not
	// be readable by other users even if the parent is.
	info, err := os.Stat(c.StateDir)
	if err != nil {
		t.Fatalf("stat state dir: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("state dir mode = %o, want 700", got)
	}
}

func TestEnsureDirsIsIdempotent(t *testing.T) {
	base := t.TempDir()
	c := &Config{
		StackDir:  filepath.Join(base, "stack"),
		BackupDir: filepath.Join(base, "backups"),
		ConfigDir: filepath.Join(base, "config"),
		StateDir:  filepath.Join(base, "state"),
	}

	for i := 0; i < 2; i++ {
		if err := EnsureDirs(c); err != nil {
			t.Fatalf("EnsureDirs run %d: %v", i+1, err)
		}
	}
}

func TestEnsureDirsReportsBlockedPath(t *testing.T) {
	base := t.TempDir()
	// A regular file where the stack directory should be makes MkdirAll fail.
	blocked := filepath.Join(base, "stack")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	c := &Config{
		StackDir:  blocked,
		BackupDir: filepath.Join(base, "backups"),
		ConfigDir: filepath.Join(base, "config"),
		StateDir:  filepath.Join(base, "state"),
	}

	if err := EnsureDirs(c); err == nil {
		t.Fatal("EnsureDirs returned nil when a directory path was a regular file")
	}
}
