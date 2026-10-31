package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune unused Orvyn related images",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		fmt.Println("[PRUNE] Pruning unused images...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "image", "prune", "-f")
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[PRUNE] Done")
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Pull latest images and restart stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		fmt.Println("[UPGRADE] Pulling images...")
		out, err := executil.RunWithTimeout(ctx, 300*time.Second, "docker", stack.ComposeArgs(cfg, "pull")...)
		if err != nil {
			return err
		}
		fmt.Print(string(out))

		fmt.Println("[UPGRADE] Starting with updated images...")
		out, err = executil.RunWithTimeout(ctx, 300*time.Second, "docker", stack.ComposeArgs(cfg, "up", "-d")...)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[UPGRADE] Done")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(upgradeCmd)
}
