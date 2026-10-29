package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alfredo-capitia/Orvyn/internal/logs"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [container|service]",
	Short: "Show logs from a container",
	Long: `Streams the logs of a container, one line at a time.

Output is demultiplexed from the Docker stream and passed through best-effort
secret redaction. Redaction is not a guarantee: see docs/THREAT-MODEL.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetString("tail")
		timestamps, _ := cmd.Flags().GetBool("timestamps")

		if len(args) == 0 {
			return fmt.Errorf("container or service name required (e.g. orvyn logs loki)")
		}

		if err := logs.Stream(ctx, os.Stdout, args[0], follow, tail, timestamps); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		return nil
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().StringP("tail", "n", "100", "Number of lines to show from the end of the logs")
	logsCmd.Flags().BoolP("timestamps", "t", false, "Show timestamps")
	rootCmd.AddCommand(logsCmd)
}
