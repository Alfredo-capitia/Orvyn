package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/backup"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	base := t.TempDir()
	return &config.Config{
		StackDir:  filepath.Join(base, "stack"),
		ConfigDir: filepath.Join(base, "config"),
		BackupDir: filepath.Join(base, "backups"),
		StateDir:  filepath.Join(base, "state"),
	}
}

func writeTestFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	cfg := testConfig(t)
	compose := filepath.Join(cfg.StackDir, "docker-compose.yml")
	conf := filepath.Join(cfg.ConfigDir, "config.yaml")
	writeTestFile(t, compose, "services:\n  loki: {}\n")
	writeTestFile(t, conf, "stack: here\n")

	var out bytes.Buffer
	archive, err := runBackup(cfg, &out)
	if err != nil {
		t.Fatalf("runBackup: %v", err)
	}
	if archive == "" {
		t.Fatal("runBackup returned no path")
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive missing: %v", err)
	}

	// Simulate an apply that clobbered the files.
	writeTestFile(t, compose, "services:\n  broken: {}\n")
	if err := os.Remove(conf); err != nil {
		t.Fatal(err)
	}

	var restoreOut bytes.Buffer
	files, err := runRestore(archive, restoreDests(cfg), &restoreOut)
	if err != nil {
		t.Fatalf("runRestore: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("restored %d files, want 2: %v", len(files), files)
	}

	got, err := os.ReadFile(compose)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "loki") {
		t.Errorf("compose was not restored: %q", got)
	}
	got, err = os.ReadFile(conf)
	if err != nil {
		t.Fatalf("config was not recreated: %v", err)
	}
	if string(got) != "stack: here\n" {
		t.Errorf("config content = %q", got)
	}
}

func TestRunBackupReportsNothingToDo(t *testing.T) {
	cfg := testConfig(t)
	var out bytes.Buffer

	path, err := runBackup(cfg, &out)
	if err != nil {
		t.Fatalf("runBackup on empty dirs: %v", err)
	}
	if path != "" {
		t.Errorf("got an archive for empty dirs: %s", path)
	}
	if !strings.Contains(out.String(), "Nothing to back up") {
		t.Errorf("missing explanation, got %q", out.String())
	}
}

func TestListBackupsOutput(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "x")
	if _, err := runBackup(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := listBackups(cfg, &out); err != nil {
		t.Fatalf("listBackups: %v", err)
	}
	for _, want := range []string{"NAME", "SIZE", "CREATED", "backup-"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("listing is missing %q: %s", want, out.String())
		}
	}
}

func TestListBackupsEmpty(t *testing.T) {
	cfg := testConfig(t)
	var out bytes.Buffer
	if err := listBackups(cfg, &out); err != nil {
		t.Fatalf("listBackups: %v", err)
	}
	if !strings.Contains(out.String(), "No backups") {
		t.Errorf("expected an empty-state message, got %q", out.String())
	}
}

func TestSelectBackup(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "one")
	if _, err := runBackup(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(cfg.StackDir, "extra.yml"), "two")
	if _, err := runBackup(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	infos, err := backup.List(cfg.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(infos))
	}

	latest, err := selectBackup(cfg, nil, true)
	if err != nil {
		t.Fatalf("selectBackup(--latest): %v", err)
	}
	if latest != infos[0].Path {
		t.Errorf("--latest chose %s, want %s", latest, infos[0].Path)
	}

	byName := filepath.Base(infos[1].Path)
	got, err := selectBackup(cfg, []string{byName}, false)
	if err != nil {
		t.Fatalf("selectBackup(name): %v", err)
	}
	if filepath.Base(got) != byName {
		t.Errorf("selectBackup(name) = %s, want %s", got, byName)
	}

	if _, err := selectBackup(cfg, []string{byName}, true); err == nil {
		t.Error("selectBackup accepted both a name and --latest")
	}
}

func TestSelectBackupWithoutChooserListsOptions(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "one")
	archive, err := runBackup(cfg, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = selectBackup(cfg, nil, false)
	if err == nil {
		t.Fatal("selectBackup without a choice should fail")
	}
	if !strings.Contains(err.Error(), filepath.Base(archive)) {
		t.Errorf("error does not list the available backup: %v", err)
	}
}

func TestSelectBackupRejectsPathsOutsideBackupDir(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "one")
	if _, err := runBackup(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(filepath.Dir(cfg.BackupDir), "secret.tar.gz")
	writeTestFile(t, outside, "not an archive")

	for _, arg := range []string{"../secret.tar.gz", outside} {
		if _, err := selectBackup(cfg, []string{arg}, false); err == nil {
			t.Errorf("selectBackup accepted %q, which is outside %s", arg, cfg.BackupDir)
		}
	}
}

func TestSelectBackupNoBackups(t *testing.T) {
	cfg := testConfig(t)
	if _, err := selectBackup(cfg, nil, true); err == nil {
		t.Error("expected an error when no backups exist")
	}
}

func TestSelectBackupRejectsSymlinkOutsideBackupDir(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "one")
	if _, err := runBackup(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "outside.tar.gz")
	writeTestFile(t, outside, "not an archive")
	link := filepath.Join(cfg.BackupDir, "backup-20260101T000000Z.tar.gz")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if _, err := selectBackup(cfg, []string{filepath.Base(link)}, false); err == nil {
		t.Error("selectBackup accepted a symlink pointing outside the backup directory")
	}
	// --latest must go through the same check, not return List's path as-is.
	if _, err := selectBackup(cfg, nil, true); err == nil {
		t.Error("selectBackup --latest accepted a symlink pointing outside the backup directory")
	}
}

func TestConfirmRestore(t *testing.T) {
	dests := map[string]string{"stack": "/tmp/stack", "config": "/tmp/config"}

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes", "y\n", true},
		{"yes upper", "Y\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"eof", "", false},
		{"garbage", "sure\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirmRestore(strings.NewReader(tt.input), &out, "/backups/backup-x.tar.gz", dests)
			if err != nil {
				t.Fatalf("confirmRestore: %v", err)
			}
			if got != tt.want {
				t.Errorf("confirmRestore(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if !strings.Contains(out.String(), "backup-x.tar.gz") {
				t.Errorf("prompt does not name the archive: %s", out.String())
			}
		})
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		0:       "0 B",
		512:     "512 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		1 << 20: "1.0 MiB",
		3 << 20: "3.0 MiB",
		5 << 30: "5.0 GiB",
	}
	for in, want := range cases {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestRunRestoreRejectsCorruptArchive(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "x")
	archive, err := runBackup(cfg, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, data[:len(data)/2], 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := runRestore(archive, restoreDests(cfg), &bytes.Buffer{}); err == nil {
		t.Fatal("runRestore accepted a truncated archive")
	}
}

func TestRunRestoreRejectsUnmanagedEntry(t *testing.T) {
	cfg := testConfig(t)
	writeTestFile(t, filepath.Join(cfg.StackDir, "docker-compose.yml"), "x")
	archive, err := runBackup(cfg, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runRestore(archive, map[string]string{"other": t.TempDir()}, &bytes.Buffer{}); !errors.Is(err, backup.ErrUnsafePath) {
		t.Fatalf("runRestore with a foreign root = %v, want ErrUnsafePath", err)
	}
}
