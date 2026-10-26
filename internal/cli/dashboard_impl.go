package cli

import (
	"fmt"
	"os"

	"github.com/fredy-arias/orvyn/internal/tui"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open Orvyn terminal dashboard (TUI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !isTTY() {
			return fmt.Errorf("dashboard requires an interactive terminal")
		}
		return tui.Run()
	},
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
