package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove Orvyn stack (same as down)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		volumes, _ := cmd.Flags().GetBool("volumes")
		argsCmd := []string{"compose", "-f", cfg.StackCompose(), "down"}
		if volumes {
			argsCmd = append(argsCmd, "-v")
		}

		fmt.Println("[RM] Removing Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", argsCmd...)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[RM] Done")
		return nil
	},
}

func init() {
	rmCmd.Flags().Bool("volumes", false, "Remove volumes as well")
	rootCmd.AddCommand(rmCmd)
}
