package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	applytrack "github.com/fredy-arias/orvyn/internal/apply"
	"github.com/fredy-arias/orvyn/internal/audit"
	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/planner"
	"github.com/fredy-arias/orvyn/internal/stack"
	"github.com/fredy-arias/orvyn/internal/validate"
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

		// Pre-flight
		pf := validate.Preflight(ctx)
		pf.Print()
		if !pf.OK {
			fmt.Println("\n[PRE-FLIGHT] Required checks failed. Aborting.")
			logger.Log(audit.Event{Operation: "apply", Result: "preflight_failed"})
			return fmt.Errorf("preflight failed")
		}

		// Show plan first
		plan, err := planner.Generate(ctx)
		if err != nil {
			return err
		}

		fmt.Println("\n=== ORVYN APPLY ===")
		fmt.Println("[PLAN]")
		fmt.Println("  Plan summary:")
		for i, a := range plan.Actions {
			fmt.Printf("    %d. [%s] %s\n", i+1, a.Type, a.Description)
		}
		fmt.Printf("\n  Stack will be created in: %s\n", cfg.StackDir)
		fmt.Println("  This will NOT modify existing docker-compose.yml files")

		// Check partial session
		if applytrack.HasPartial() {
			if s, err := applytrack.Load(); err == nil {
				applytrack.PrintRecovery(s)
			}
		}

		fmt.Println("\n[CONFIRM]")
		fmt.Println("  Type 'y' to confirm, anything else to abort:")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" {
			fmt.Println("  Aborted.")
			logger.Log(audit.Event{Operation: "apply", Result: "aborted"})
			return nil
		}

		sess := applytrack.NewSession()
		sess.Compose = cfg.StackCompose()
		sess.AddStep("preflight", "done", "-")
		if err := sess.Save(); err != nil {
			return err
		}

		fmt.Println("\n[FILES]")
		sess.AddStep("ensure_dirs", "in_progress", cfg.StackDir)
		sess.Save()
		if err := config.EnsureDirs(cfg); err != nil {
			sess.AddStep("ensure_dirs", "failed", cfg.StackDir)
			sess.Status = "failed"
			sess.Save()
			applytrack.PrintRecovery(sess)
			logger.Log(audit.Event{Operation: "apply", Result: "failed"})
			return err
		}
		sess.AddStep("ensure_dirs", "done", cfg.StackDir)
		sess.AddCreated(cfg.StackDir)
		sess.Save()

		sess.AddStep("generate_stack", "in_progress", cfg.StackDir)
		sess.Save()
		if err := stack.GenerateAll(); err != nil {
			sess.AddStep("generate_stack", "failed", cfg.StackDir)
			sess.Status = "failed"
			sess.Save()
			applytrack.PrintRecovery(sess)
			logger.Log(audit.Event{Operation: "apply", Result: "failed"})
			return err
		}
		sess.AddStep("generate_stack", "done", cfg.StackDir)
		sess.AddCreated(cfg.StackCompose())
		sess.AddCreated(cfg.StackDir + "/prometheus/prometheus.yml")
		sess.AddCreated(cfg.StackDir + "/loki/loki-config.yml")
		sess.AddCreated(cfg.StackDir + "/alloy/config.alloy")
		sess.Save()
		fmt.Printf("  [OK] Written stack files to %s\n", cfg.StackDir)

		fmt.Println("\n[IMAGES/SERVICES]")
		sess.AddStep("docker_compose_up", "in_progress", cfg.StackCompose())
		sess.Save()
		c := exec.CommandContext(ctx, "docker", "compose", "-f", cfg.StackCompose(), "up", "-d")
		out, err := c.CombinedOutput()
		if err != nil {
			sess.Steps[len(sess.Steps)-1].Err = string(out)
			sess.AddStep("docker_compose_up", "failed", cfg.StackCompose())
			sess.Status = "failed"
			sess.Save()
			fmt.Printf("  [FAIL] docker compose up -d failed\n")
			applytrack.PrintRecovery(sess)
			logger.Log(audit.Event{Operation: "apply", Result: "failed"})
			return err
		}
		sess.AddStep("docker_compose_up", "done", cfg.StackCompose())
		sess.Save()
		fmt.Printf("  [OK] %s\n", strings.TrimSpace(string(out)))

		sess.Status = "completed"
		sess.Save()
		_ = applytrack.Clear()

		fmt.Println("\n[HEALTH]")
		fmt.Println("  [PENDING] Run 'orvyn status' or 'orvyn validate' to check service health")
		fmt.Printf("\nStack started in %s\n", cfg.StackDir)
		fmt.Println("Access Prometheus: http://127.0.0.1:9090 (localhost only)")
		fmt.Println("Access Loki: http://127.0.0.1:3100 (localhost only)")

		logger.Log(audit.Event{Operation: "apply", Approved: true, Result: "ok", Target: cfg.StackDir})
		_ = time.Now()
		return nil
	},
}
