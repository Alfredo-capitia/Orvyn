package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
	"github.com/spf13/cobra"
)

func statusCmdRun(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := docker.New()
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer c.Close()

	containers, err := discovery.ManagedStack(ctx)
	if err != nil {
		return fmt.Errorf("list managed: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No Orvyn managed containers found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSERVICE\tSTATUS\tSTATE\tHEALTH")
	for _, ct := range containers {
		name := ct.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		svc := ct.Labels["com.orvyn.service"]
		if svc == "" {
			svc = "-"
		}
		health := "-"
		if ct.State == "running" {
			health = "unknown"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, svc, ct.Status, ct.State, health)
	}
	w.Flush()
	return nil
}

func init() {
	statusCmd.RunE = statusCmdRun
}
