package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/backup"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [backup]",
	Short: "Restore the Orvyn stack and configuration from a backup",
	Long: `Repopulates the stack and configuration directories from a backup
archive created by 'orvyn backup'.

The archive is fully validated before anything is written, and every entry is
confined to the destination directory. Existing files that are present in the
archive are overwritten, so a confirmation is required unless --yes is given.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		latest, _ := cmd.Flags().GetBool("latest")
		assumeYes, _ := cmd.Flags().GetBool("yes")

		archive, err := selectBackup(cfg, args, latest)
		if err != nil {
			return err
		}

		dests := restoreDests(cfg)
		if !assumeYes {
			ok, err := confirmRestore(os.Stdin, os.Stdout, archive, dests)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted.")
				audit.New(cfg.StateDir + "/audit.jsonl").Log(audit.Event{Operation: "restore", Result: "aborted"})
				return nil
			}
		}

		_, err = runRestore(archive, dests, os.Stdout)
		logger := audit.New(cfg.StateDir + "/audit.jsonl")
		if err != nil {
			logger.Log(audit.Event{Operation: "restore", Result: "failed", Target: archive})
			return err
		}
		logger.Log(audit.Event{Operation: "restore", Approved: true, Result: "ok", Target: archive})
		return nil
	},
}

// restoreDests maps archive top-level names to their destination directories.
// It must mirror backupRoots.
func restoreDests(cfg *config.Config) map[string]string {
	return map[string]string{
		"stack":  cfg.StackDir,
		"config": cfg.ConfigDir,
	}
}

func runRestore(archive string, dests map[string]string, out io.Writer) ([]string, error) {
	files, err := backup.Restore(archive, dests)
	if err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}
	fmt.Fprintf(out, "Restored %d file(s) from %s\n", len(files), filepath.Base(archive))
	for _, f := range files {
		fmt.Fprintf(out, "  - %s\n", f)
	}
	return files, nil
}

// selectBackup resolves which archive to restore. The chosen path is always
// validated by validateArchive, including when it comes from --latest, so a
// symlink inside the backup directory cannot point outside it.
func selectBackup(cfg *config.Config, args []string, latest bool) (string, error) {
	if len(args) > 0 && latest {
		return "", errors.New("--latest cannot be combined with an explicit backup")
	}

	if len(args) == 1 {
		candidate := args[0]
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(cfg.BackupDir, candidate)
		}
		return validateArchive(cfg, candidate, args[0])
	}

	infos, err := backup.List(cfg.BackupDir)
	if err != nil {
		return "", fmt.Errorf("list backups: %w", err)
	}
	if len(infos) == 0 {
		return "", fmt.Errorf("no backups found in %s", cfg.BackupDir)
	}
	if latest {
		return validateArchive(cfg, infos[0].Path, filepath.Base(infos[0].Path))
	}

	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, filepath.Base(info.Path))
	}
	return "", fmt.Errorf("choose a backup to restore, or use --latest\navailable:\n  %s",
		strings.Join(names, "\n  "))
}

// validateArchive resolves both the archive and the backup directory through
// symlinks before checking containment, and requires a regular file. Lexical
// comparison alone would accept a link inside the backup directory that points
// anywhere on disk.
func validateArchive(cfg *config.Config, path, label string) (string, error) {
	base, err := filepath.EvalSymlinks(cfg.BackupDir)
	if err != nil {
		return "", fmt.Errorf("backup directory %s is not accessible: %w", cfg.BackupDir, err)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("backup not found: %s", label)
	}
	if resolved != base && !strings.HasPrefix(resolved, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("%q is outside the backup directory %s", label, base)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular file", label)
	}
	return resolved, nil
}

func confirmRestore(in io.Reader, out io.Writer, archive string, dests map[string]string) (bool, error) {
	targets := make([]string, 0, len(dests))
	for name, dir := range dests {
		targets = append(targets, fmt.Sprintf("  %-8s -> %s", name, dir))
	}
	sort.Strings(targets)

	fmt.Fprintf(out, "\n=== ORVYN RESTORE ===\nArchive: %s\nThis overwrites files in:\n%s\n",
		filepath.Base(archive), strings.Join(targets, "\n"))
	fmt.Fprintln(out, "\nType 'y' to confirm, anything else to abort:")

	input, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.ToLower(strings.TrimSpace(input)) == "y", nil
}

func init() {
	restoreCmd.Flags().Bool("latest", false, "Restore the newest backup")
	restoreCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(restoreCmd)
}
