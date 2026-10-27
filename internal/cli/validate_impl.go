package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/Alfredo-capitia/Orvyn/internal/docker"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate observability stack health",
}

func validateCmdRun(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("[VALIDATE] Docker")
	c, err := docker.New()
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
		return err
	}
	defer c.Close()
	if err := c.Ping(ctx); err != nil {
		fmt.Printf("  FAIL: ping: %v\n", err)
		return err
	}
	fmt.Println("  OK")

	fmt.Println("[VALIDATE] Managed stack")
	containers, err := discovery.ManagedStack(ctx)
	if err != nil {
		fmt.Printf("  FAIL: %v\n", err)
		return err
	}
	if len(containers) == 0 {
		fmt.Println("  WARN: no managed containers found")
	} else {
		fmt.Printf("  OK: %d container(s) found\n", len(containers))
		for _, ct := range containers {
			name := ct.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			fmt.Printf("    - %s: %s (%s)\n", name, ct.State, ct.Status)
		}
	}

	fmt.Println("[VALIDATE] Endpoints")
	client := &http.Client{Timeout: 5 * time.Second}
	check := func(url string) {
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("  FAIL: %s: %v\n", url, err)
			return
		}
		resp.Body.Close()
		fmt.Printf("  OK: %s -> %d\n", url, resp.StatusCode)
	}
	check("http://127.0.0.1:9090/-/ready")
	check("http://127.0.0.1:3100/ready")

	fmt.Println("[VALIDATE] Done")
	return nil
}

func init() {
	validateCmd.RunE = validateCmdRun
}
