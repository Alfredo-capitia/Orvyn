package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	StackDir  string
	BackupDir string
	ConfigDir string
	StateDir  string
}

func Default() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	// If running as root, use system paths
	if os.Geteuid() == 0 {
		return &Config{
			StackDir:  "/opt/orvyn/stack",
			BackupDir: "/opt/orvyn/backups",
			ConfigDir: "/etc/orvyn",
			StateDir:  "/var/lib/orvyn",
		}
	}

	// Non-root: use user-local paths (no sudo needed)
	base := filepath.Join(home, ".local", "share", "orvyn")
	return &Config{
		StackDir:  filepath.Join(base, "stack"),
		BackupDir: filepath.Join(base, "backups"),
		ConfigDir: filepath.Join(home, ".config", "orvyn"),
		StateDir:  filepath.Join(home, ".local", "state", "orvyn"),
	}
}

func EnsureDirs(c *Config) error {
	dirs := []string{c.StackDir, c.BackupDir, c.ConfigDir, c.StateDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0750); err != nil {
			return err
		}
	}
	if err := os.Chmod(c.StateDir, 0700); err != nil {
		// best effort
	}
	return nil
}

func (c *Config) StackCompose() string {
	return filepath.Join(c.StackDir, "docker-compose.yml")
}
