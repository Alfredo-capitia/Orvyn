package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [container|service]",
	Short: "Show logs from a container",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetString("tail")
		timestamps, _ := cmd.Flags().GetBool("timestamps")

		if len(args) == 0 {
			return fmt.Errorf("container or service name required (e.g. orvyn logs loki)")
		}
		target := args[0]

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

		rc, err := dc.ContainerLogs(ctx, id, follow, tail, timestamps)
		if err != nil {
			return err
		}
		defer rc.Close()

		_, err = os.Stdout.ReadFrom(rc)
		if err != nil && err != context.Canceled {
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
