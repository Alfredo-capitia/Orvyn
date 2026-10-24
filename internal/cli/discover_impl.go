package cli

import (
	"context"
	"fmt"

	"github.com/fredy-arias/vpsmon/internal/discovery"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover environment (read-only)",
	Long:  "Scans the system for Docker, containers, and existing observability stack without making changes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		res, err := discovery.Run(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("OS: %s\n", res.OS)
		fmt.Printf("Docker available: %v\n", res.DockerAvailable)
		if res.DockerInfo != nil {
			if v, ok := res.DockerInfo["ServerVersion"]; ok {
				fmt.Printf("Docker version: %v\n", v)
			}
			if r, ok := res.DockerInfo["ContainersRunning"]; ok {
				fmt.Printf("Containers running: %v\n", r)
			}
		}
		fmt.Printf("\nContainers (%d):\n", len(res.Containers))
		for _, c := range res.Containers {
			name := ""
			if len(c.Names) > 0 {
				name = c.Names[0]
				if len(name) > 0 && name[0] == '/' {
					name = name[1:]
				}
			}
			fmt.Printf("  - %s [%s] %s (%s)\n", name, c.ID, c.Image, c.State)
		}
		if len(res.ExistingStack) > 0 {
			fmt.Printf("\nExisting observability stack detected:\n")
			for _, s := range res.ExistingStack {
				fmt.Printf("  - %s\n", s)
			}
		} else {
			fmt.Printf("\nExisting observability stack detected: none\n")
		}
		fmt.Printf("\n[read-only] No changes made.\n")
		return nil
	},
}
