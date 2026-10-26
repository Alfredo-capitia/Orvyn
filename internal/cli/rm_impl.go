package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
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

		argsCmd := []string{"compose", "-f", cfg.StackCompose(), "down"}
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
