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
