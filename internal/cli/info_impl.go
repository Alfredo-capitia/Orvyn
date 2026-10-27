package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show Orvyn environment info",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Println("Orvyn Info")
		fmt.Printf("  Version:   v0.0.1-dev\n")
		fmt.Printf("  Config:    %s\n", cfg.ConfigDir)
		fmt.Printf("  Stack:     %s\n", cfg.StackDir)
		fmt.Printf("  Backups:   %s\n", cfg.BackupDir)
		fmt.Printf("  State:     %s\n", cfg.StateDir)
		fmt.Printf("  Compose:   %s\n", cfg.StackCompose())

		managed, err := discovery.ManagedStack(ctx)
		if err != nil {
			fmt.Printf("  Managed:   error: %v\n", err)
			return nil
		}
		running := 0
		for _, ct := range managed {
			if ct.State == "running" {
				running++
			}
		}
		fmt.Printf("  Managed:   %d container(s) (%d running)\n", len(managed), running)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
