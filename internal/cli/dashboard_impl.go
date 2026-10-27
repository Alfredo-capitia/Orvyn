package cli

import (
	"fmt"
	"os"

	"github.com/Alfredo-capitia/Orvyn/internal/tui"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open Orvyn terminal dashboard (TUI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("CI") != "" || os.Getenv("ORVYN_NO_TUI") != "" {
			return fmt.Errorf("dashboard is disabled in non-interactive environments")
		}
		return tui.Run()
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
