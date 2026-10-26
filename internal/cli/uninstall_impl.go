package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Orvyn stack completely",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
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

		if !force {
			fmt.Printf("This will remove Orvyn containers and network using %s\n", cfg.StackCompose())
			if volumes {
				fmt.Println("VOLUMES WILL BE REMOVED - ALL STACK DATA WILL BE LOST")
			} else {
				fmt.Println("Volumes will be preserved")
			}
			fmt.Print("Type 'y' to confirm: ")
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			if strings.TrimSpace(text) != "y" {
				fmt.Println("Aborted")
				return nil
			}
		}

		argsCmd := []string{"compose", "-f", cfg.StackCompose(), "down"}
		if volumes {
			argsCmd = append(argsCmd, "-v")
		}

		fmt.Println("[UNINSTALL] Removing stack...")
		out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", argsCmd...)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
		} else {
			fmt.Print(string(out))
		}
		fmt.Println("[UNINSTALL] Done")
		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	uninstallCmd.Flags().Bool("volumes", false, "Remove volumes as well")
	rootCmd.AddCommand(uninstallCmd)
}
