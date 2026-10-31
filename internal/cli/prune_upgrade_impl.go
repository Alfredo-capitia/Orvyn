package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/Alfredo-capitia/Orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune unused Docker images",
	Long: `Runs 'docker image prune' against the local Docker daemon.

The prune is not restricted to Orvyn: it removes every unused image Docker
considers removable, so a confirmation is required unless --yes is given.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		logger := audit.New(cfg.StateDir + "/audit.jsonl")
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		assumeYes, _ := cmd.Flags().GetBool("yes")
		if !assumeYes {
			lines := []string{
				"Command: docker image prune -f",
				"This removes unused images for the whole Docker daemon, not only Orvyn images.",
			}
			ok, err := confirmDestructive(os.Stdin, os.Stdout, "ORVYN PRUNE", lines)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted.")
				logger.Log(audit.Event{Operation: "prune", Result: "aborted"})
				return nil
			}
		}

		fmt.Println("[PRUNE] Pruning unused images...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "image", "prune", "-f")
		if err != nil {
			logger.Log(audit.Event{Operation: "prune", Result: "failed"})
			return err
		}
		fmt.Print(string(out))
		logger.Log(audit.Event{Operation: "prune", Approved: true, Result: "ok"})
		fmt.Println("[PRUNE] Done")
		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Pull latest images and restart stack",
	Long: `Pulls the newest images for the stack and recreates its containers.

Containers are recreated, so running services restart. A confirmation is
required unless --yes is given.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		logger := audit.New(cfg.StateDir + "/audit.jsonl")
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		assumeYes, _ := cmd.Flags().GetBool("yes")
		if !assumeYes {
			lines := []string{
				"Stack: " + cfg.StackCompose(),
				"The newest images will be pulled and the containers recreated.",
				"Running services will restart.",
			}
			ok, err := confirmDestructive(os.Stdin, os.Stdout, "ORVYN UPGRADE", lines)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted.")
				logger.Log(audit.Event{Operation: "upgrade", Result: "aborted", Target: cfg.StackDir})
				return nil
			}
		}

		fmt.Println("[UPGRADE] Pulling images...")
		out, err := executil.RunWithTimeout(ctx, 300*time.Second, "docker", stack.ComposeArgs(cfg, "pull")...)
		if err != nil {
			logger.Log(audit.Event{Operation: "upgrade", Result: "failed", Target: cfg.StackDir})
			return err
		}
		fmt.Print(string(out))

		fmt.Println("[UPGRADE] Starting with updated images...")
		out, err = executil.RunWithTimeout(ctx, 300*time.Second, "docker", stack.ComposeArgs(cfg, "up", "-d")...)
		if err != nil {
			logger.Log(audit.Event{Operation: "upgrade", Result: "failed", Target: cfg.StackDir})
			return err
		}
		fmt.Print(string(out))
		logger.Log(audit.Event{Operation: "upgrade", Approved: true, Result: "ok", Target: cfg.StackDir})
		fmt.Println("[UPGRADE] Done")
		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
	upgradeCmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(upgradeCmd)
}
