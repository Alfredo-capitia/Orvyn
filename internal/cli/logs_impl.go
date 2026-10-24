package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fredy-arias/vpsmon/internal/logs"
	"github.com/spf13/cobra"
)

var follow bool
var tail string
var timestamps bool

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "Follow logs for a service (read-only)",
	Long:  "Stream logs from containers using Docker API with automatic secret redaction.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return logs.Stream(ctx, args[0], follow, tail, timestamps)
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().StringVar(&tail, "tail", "", "Number of lines to show from the end of the logs")
	logsCmd.Flags().BoolVarP(&timestamps, "timestamps", "t", false, "Show timestamps")
}
