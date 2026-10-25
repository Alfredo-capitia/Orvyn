package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of managed components",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("status: not implemented yet (will be implemented in Fase 7)")
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate observability stack health",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("validate: not implemented yet (will be implemented in Fase 7)")
	},
}
