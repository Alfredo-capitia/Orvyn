package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fredy-arias/orvyn/internal/audit"
	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/planner"
	"github.com/fredy-arias/orvyn/internal/stack"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply the planned configuration",
	Long:  "Applies changes only after explicit confirmation. Creates backups before modifications.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg := config.Default()
		logger := audit.New(cfg.StateDir + "/audit.jsonl")

		// Show plan first
		plan, err := planner.Generate(ctx)
		if err != nil {
			return err
		}

		fmt.Println("=== Orvyn Apply ===")
		fmt.Println("Plan summary:")
		for i, a := range plan.Actions {
			fmt.Printf("  %d. [%s] %s\n", i+1, a.Type, a.Description)
		}
		fmt.Printf("\nStack will be created in: %s\n", cfg.StackDir)
		fmt.Println("This will NOT modify existing docker-compose.yml files")
		fmt.Println("\nType 'y' to confirm, anything else to abort:")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" {
			fmt.Println("Aborted.")
			logger.Log(audit.Event{Operation: "apply", Result: "aborted"})
			return nil
		}

		fmt.Println("\n[1/4] Ensuring directories...")
		if err := config.EnsureDirs(cfg); err != nil {
			return err
		}

		fmt.Println("[2/4] Generating stack files...")
		if err := stack.GenerateAll(); err != nil {
			return err
		}

		fmt.Println("[3/4] Starting stack with docker compose...")
		c := exec.CommandContext(ctx, "docker", "compose", "-f", cfg.StackCompose(), "up", "-d")
		out, err := c.CombinedOutput()
		if err != nil {
			fmt.Printf("Error: %s\n%s\n", err, string(out))
			logger.Log(audit.Event{Operation: "apply", Result: "failed"})
			return err
		}
		fmt.Println(string(out))

		fmt.Println("[4/4] Done")
		fmt.Printf("\nStack started in %s\n", cfg.StackDir)
		fmt.Println("Run 'orvyn status' or 'orvyn validate' to check health.")
		fmt.Println("Access Prometheus: http://127.0.0.1:9090 (localhost only)")
		fmt.Println("Access Loki: http://127.0.0.1:3100 (localhost only)")

		logger.Log(audit.Event{Operation: "apply", Approved: true, Result: "ok", Target: cfg.StackDir})
		_ = time.Now()
		return nil
	},
}
