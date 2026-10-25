package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
	"github.com/fredy-arias/orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <service> [command...]",
	Short: "Execute a command in a managed container",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		cmdArgs := args[1:]
		if len(cmdArgs) == 0 {
			cmdArgs = []string{"sh"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		dc, err := docker.New()
		if err != nil {
			return err
		}
		defer dc.Close()

		var cID string
		if found, err := dc.FindContainerByNameOrService(ctx, target); err == nil && found != nil {
			cID = found.ID
		} else {
			if managed, err := discovery.ManagedStack(ctx); err == nil {
				for _, ct := range managed {
					name := ct.Names[0]
					if len(name) > 0 && name[0] == '/' {
						name = name[1:]
					}
					svc := ct.Labels["com.orvyn.service"]
					if name == target || svc == target || name == "orvyn-"+target {
						cID = ct.ID
						break
					}
				}
			}
		}

		if cID == "" {
			return fmt.Errorf("container/service not found: %s", target)
		}

		id := cID
		if len(id) > 12 {
			id = id[:12]
		}

		execArgs := append([]string{"exec", "-it", id}, cmdArgs...)
		// fallback to non-interactive if not a tty
		if !isTerminal() {
			execArgs = []string{"exec", "-i", id}
			execArgs = append(execArgs, cmdArgs...)
		}

		fmt.Printf("[EXEC] %s\n", target)
		out, err := executil.RunWithTimeout(context.Background(), 300*time.Second, "docker", execArgs...)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		return nil
	},
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate shell completion scripts",
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(completionCmd)
}
