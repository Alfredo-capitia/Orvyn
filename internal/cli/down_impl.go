package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove Orvyn stack (volumes preserved by default)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		volumes, _ := cmd.Flags().GetBool("volumes")

		if _, err := os.Stat(cfg.StackCompose()); os.IsNotExist(err) {
			return fmt.Errorf("stack compose not found: %s (nothing to down)", cfg.StackCompose())
		}

		managed, err := discovery.ManagedStack(ctx)
		if err == nil && len(managed) > 0 {
			fmt.Printf("[DOWN] Targeting %d Orvyn-managed container(s):\n", len(managed))
			for _, ct := range managed {
				name := ct.Names[0]
				if len(name) > 0 && name[0] == '/' {
					name = name[1:]
				}
				svc := ct.Labels["com.orvyn.service"]
				if svc == "" {
					svc = "-"
				}
				fmt.Printf("  - %s [%s] %s\n", name, svc, ct.State)
			}
			fmt.Println()
		} else if err != nil {
			fmt.Printf("[DOWN] Warning: could not list managed containers: %v\n", err)
		}

		if err := verifyProjectOwnership(ctx, cfg); err != nil {
			return err
		}

		argsCmd := stack.ComposeArgs(cfg, "down")
		if volumes {
			argsCmd = append(argsCmd, "-v")
			fmt.Println("[DOWN] Volumes will be REMOVED")
		} else {
			fmt.Println("[DOWN] Volumes will be preserved")
		}
		fmt.Println()

		fmt.Println("[DOWN] Stopping Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", argsCmd...)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[DOWN] Done")
		return nil
	},
}

func init() {
	downCmd.Flags().Bool("volumes", false, "Remove volumes as well")
	rootCmd.AddCommand(downCmd)
}
