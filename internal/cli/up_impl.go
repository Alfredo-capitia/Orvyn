package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start Orvyn observability stack (idempotent)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		detached, _ := cmd.Flags().GetBool("detach")
		if detached {
			// default behavior
		}

		// Ensure stack files exist
		if _, err := os.Stat(cfg.StackCompose()); os.IsNotExist(err) {
			fmt.Println("[UP] Generating stack files...")
			if err := stack.GenerateAll(); err != nil {
				return err
			}
		}

		c, err := docker.New()
		if err == nil {
			defer c.Close()
			if c.Ping(ctx) == nil {
				if managed, err := discovery.ManagedStack(ctx); err == nil && len(managed) > 0 {
					fmt.Printf("[UP] Found %d managed containers, reconciling...\n", len(managed))
				}
			}
		}

		fmt.Println("[UP] Starting stack with docker compose up -d...")
		out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "up", "-d")
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[UP] Done")
		return nil
	},
}

func init() {
	upCmd.Flags().BoolP("detach", "d", true, "Run in background")
	rootCmd.AddCommand(upCmd)
}
