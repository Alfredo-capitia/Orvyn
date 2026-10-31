package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove Orvyn stack (same as down)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		logger := audit.New(cfg.StateDir + "/audit.jsonl")
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		volumes, _ := cmd.Flags().GetBool("volumes")
		assumeYes, _ := cmd.Flags().GetBool("yes")

		if _, err := os.Stat(cfg.StackCompose()); os.IsNotExist(err) {
			return fmt.Errorf("stack compose not found: %s (nothing to remove)", cfg.StackCompose())
		}

		managed, err := discovery.ManagedStack(ctx)
		if err == nil && len(managed) > 0 {
			fmt.Printf("[RM] Targeting %d Orvyn-managed container(s):\n", len(managed))
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
			fmt.Printf("[RM] Warning: could not list managed containers: %v\n", err)
		}

		if err := verifyProjectOwnership(ctx, cfg); err != nil {
			return err
		}

		volumeLine := "Volumes will be preserved."
		if volumes {
			volumeLine = "Volumes will be REMOVED. All stack data will be lost."
		}

		if !assumeYes {
			lines := []string{
				"Stack: " + cfg.StackCompose(),
				volumeLine,
				"This stops and removes the Orvyn containers and network.",
			}
			ok, err := confirmDestructive(os.Stdin, os.Stdout, "ORVYN RM", lines)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted.")
				logger.Log(audit.Event{Operation: "rm", Result: "aborted", Target: cfg.StackDir})
				return nil
			}
		}

		argsCmd := stack.ComposeArgs(cfg, "down")
		if volumes {
			argsCmd = append(argsCmd, "-v")
			fmt.Println("[RM] Volumes will be REMOVED")
		} else {
			fmt.Println("[RM] Volumes will be preserved")
		}
		fmt.Println()

		fmt.Println("[RM] Removing Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", argsCmd...)
		if err != nil {
			logger.Log(audit.Event{Operation: "rm", Result: "failed", Target: cfg.StackDir})
			return err
		}
		fmt.Print(string(out))
		logger.Log(audit.Event{Operation: "rm", Approved: true, Result: "ok", Target: cfg.StackDir})
		fmt.Println("[RM] Done")
		return nil
	},
}

func init() {
	rmCmd.Flags().Bool("volumes", false, "Remove volumes as well")
	rmCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
