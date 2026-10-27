package cli

import (
	"fmt"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show Orvyn configuration paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		fmt.Println("Orvyn config:")
		fmt.Printf("  Config dir: %s\n", cfg.ConfigDir)
		fmt.Printf("  Stack dir:  %s\n", cfg.StackDir)
		fmt.Printf("  Stack file: %s\n", cfg.StackCompose())
		fmt.Printf("  Backups:    %s\n", cfg.BackupDir)
		fmt.Printf("  State:      %s\n", cfg.StateDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
