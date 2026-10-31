package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:                "exec <service> [command...]",
	Short:              "Execute a command in a managed container",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
			return cmd.Help()
		}
		target := args[0]
		cmdArgs := args[1:]
		if len(cmdArgs) == 0 {
			cmdArgs = []string{"sh"}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		dc, err := docker.New()
		if err != nil {
			return err
		}
		defer dc.Close()

		found, err := dc.FindContainerByNameOrService(ctx, target)
		if err != nil {
			return err
		}
		if found == nil {
			return fmt.Errorf("managed container or service not found: %s", target)
		}
		cID := found.ID

		id := cID
		if len(id) > 12 {
			id = id[:12]
		}

		fmt.Printf("[EXEC] %s\n", target)
		dargs := []string{"exec"}
		if isTerminal() {
			dargs = append(dargs, "-it")
		} else {
			dargs = append(dargs, "-i")
		}
		dargs = append(dargs, id)
		dargs = append(dargs, cmdArgs...)

		return executil.RunInteractive(ctx, "docker", dargs...)
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
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
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
