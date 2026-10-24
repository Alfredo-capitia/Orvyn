package cli

import (
	"context"
	"fmt"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/planner"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show execution plan (read-only)",
	Long:  "Generates a detailed plan of changes without executing anything.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		plan, err := planner.Generate(ctx)
		if err != nil {
			return err
		}
		cfg := config.Default()

		fmt.Println("=== Orvyn Plan (read-only) ===")
		fmt.Printf("Created at: %s\n", plan.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Stack directory: %s\n", cfg.StackDir)
		fmt.Printf("Backup directory: %s\n\n", cfg.BackupDir)

		fmt.Println("Actions:")
		for i, a := range plan.Actions {
			fmt.Printf("  %d. [%s] %s\n", i+1, a.Type, a.Description)
			if a.Path != "-" {
				fmt.Printf("     Path: %s\n", a.Path)
			}
			fmt.Printf("     Impact: %s\n", a.Impact)
			fmt.Println()
		}

		if len(plan.Backups) > 0 {
			fmt.Println("Backups planned:")
			for _, b := range plan.Backups {
				fmt.Printf("  - %s -> %s (%s)\n", b.Original, b.Backup, b.Reason)
			}
			fmt.Println()
		}

		fmt.Println("Notes:")
		fmt.Println("  - Stack will be ISOLATED in stack directory/ (won't touch existing docker-compose.yml)")
		fmt.Println("  - No changes made yet")
		fmt.Println("  - Use 'orvyn apply' to execute (requires confirmation)")
		fmt.Println("\n[read-only] No changes made.")
		return nil
	},
}
