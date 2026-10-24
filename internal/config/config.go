package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir = "/etc/vpsmon"
	DefaultStackDir  = "/opt/vpsmon/stack"
	DefaultBackupDir = "/opt/vpsmon/backups"
)

type Config struct {
	StackDir  string
	BackupDir string
	ConfigDir string
	StateDir  string
}

func Default() *Config {
	return &Config{
		StackDir:  DefaultStackDir,
		BackupDir: DefaultBackupDir,
		ConfigDir: DefaultConfigDir,
		StateDir:  "/var/lib/vpsmon",
	}
}

func EnsureDirs(c *Config) error {
	dirs := []string{c.StackDir, c.BackupDir, c.ConfigDir, c.StateDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0750); err != nil {
			return err
		}
	}
	// tighten state dir
	if err := os.Chmod(c.StateDir, 0700); err != nil {
		// best effort
	}
	return nil
}

func (c *Config) StackCompose() string {
	return filepath.Join(c.StackDir, "docker-compose.yml")
}
