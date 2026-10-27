package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/backup"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a timestamped backup of the Orvyn stack and configuration",
	Long: `Archives the generated stack and the Orvyn configuration into
$BACKUP_DIR/backup-<timestamp>.tar.gz.

Use --list to see the existing backups.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		list, _ := cmd.Flags().GetBool("list")

		if list {
			return listBackups(cfg, os.Stdout)
		}

		path, err := runBackup(cfg, os.Stdout)
		if err != nil {
			audit.New(cfg.StateDir + "/audit.jsonl").Log(audit.Event{Operation: "backup", Result: "failed"})
			return err
		}
		audit.New(cfg.StateDir + "/audit.jsonl").Log(audit.Event{
			Operation: "backup",
			Approved:  true,
			Result:    "ok",
			Target:    path,
		})
		return nil
	},
}

// backupRoots maps the archive top-level names to the directories they contain.
// These names are part of the on-disk format and must stay stable so old
// archives keep restoring correctly.
func backupRoots(cfg *config.Config) []backup.Root {
	return []backup.Root{
		{Name: "stack", Path: cfg.StackDir},
		{Name: "config", Path: cfg.ConfigDir},
	}
}

func runBackup(cfg *config.Config, out io.Writer) (string, error) {
	path, err := backup.Create(cfg.BackupDir, backupRoots(cfg)...)
	if err != nil {
		if errors.Is(err, backup.ErrEmpty) {
			fmt.Fprintln(out, "Nothing to back up yet: the stack and config directories are empty.")
			return "", nil
		}
		return "", fmt.Errorf("backup: %w", err)
	}

	info, statErr := os.Stat(path)
	size := int64(0)
	if statErr == nil {
		size = info.Size()
	}
	fmt.Fprintf(out, "Backup written: %s (%s)\n", path, humanSize(size))
	return path, nil
}

func listBackups(cfg *config.Config, out io.Writer) error {
	infos, err := backup.List(cfg.BackupDir)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	if len(infos) == 0 {
		fmt.Fprintf(out, "No backups in %s\n", cfg.BackupDir)
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tCREATED")
	for _, info := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			filepath.Base(info.Path),
			humanSize(info.Size),
			info.ModTime.Local().Format("2006-01-02 15:04:05"),
		)
	}
	return w.Flush()
}

// humanSize renders a byte count with one decimal place in binary units.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 3; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

func init() {
	backupCmd.Flags().Bool("list", false, "List existing backups instead of creating one")
	rootCmd.AddCommand(backupCmd)
}
