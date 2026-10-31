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

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Orvyn stack completely",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		logger := audit.New(cfg.StateDir + "/audit.jsonl")
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		force, _ := cmd.Flags().GetBool("force")
		volumes, _ := cmd.Flags().GetBool("volumes")

		if _, err := os.Stat(cfg.StackCompose()); os.IsNotExist(err) {
			return fmt.Errorf("stack compose not found: %s (nothing to uninstall)", cfg.StackCompose())
		}

		managed, err := discovery.ManagedStack(ctx)
		if err == nil && len(managed) > 0 {
			fmt.Printf("[UNINSTALL] Targeting %d Orvyn-managed container(s):\n", len(managed))
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
			fmt.Printf("[UNINSTALL] Warning: could not list managed containers: %v\n", err)
		}

		if err := verifyProjectOwnership(ctx, cfg); err != nil {
			return err
		}

		if !force {
			lines := []string{
				"Stack: " + cfg.StackCompose(),
				"Volumes will be preserved.",
				"This removes the Orvyn containers and network.",
			}
			if volumes {
				lines[1] = "Volumes will be REMOVED. All stack data will be lost."
			}
			ok, err := confirmDestructive(os.Stdin, os.Stdout, "ORVYN UNINSTALL", lines)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted")
				logger.Log(audit.Event{Operation: "uninstall", Result: "aborted", Target: cfg.StackDir})
				return nil
			}
		}

		argsCmd := stack.ComposeArgs(cfg, "down")
		if volumes {
			argsCmd = append(argsCmd, "-v")
		}

		fmt.Println("[UNINSTALL] Removing stack...")
		out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", argsCmd...)
		if err != nil {
			logger.Log(audit.Event{Operation: "uninstall", Result: "failed", Target: cfg.StackDir})
			return fmt.Errorf("remove stack: %w", err)
		}
		fmt.Print(string(out))
		logger.Log(audit.Event{Operation: "uninstall", Approved: true, Result: "ok", Target: cfg.StackDir})
		fmt.Println("[UNINSTALL] Done")
		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	uninstallCmd.Flags().Bool("volumes", false, "Remove volumes as well")
	rootCmd.AddCommand(uninstallCmd)
}
