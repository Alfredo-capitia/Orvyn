package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
	"github.com/fredy-arias/orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics for Orvyn installation",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fmt.Println("=== Orvyn Doctor ===")

		// Docker
		fmt.Println("\n[DOCKER]")
		dc, err := docker.New()
		if err != nil {
			fmt.Printf("  [FAIL] docker client: %v\n", err)
		} else {
			defer dc.Close()
			if err := dc.Ping(ctx); err != nil {
				fmt.Printf("  [FAIL] docker daemon: %v\n", err)
			} else {
				fmt.Println("  [OK]   docker daemon reachable")
			}
		}

		if out, err := executil.RunWithTimeout(ctx, 10*time.Second, "docker", "compose", "version"); err != nil {
			fmt.Printf("  [FAIL] docker compose: %v\n", err)
		} else {
			s := strings.TrimSpace(string(out))
			if s != "" {
				fmt.Printf("  [OK]   docker compose: %s\n", s)
			} else {
				fmt.Println("  [OK]   docker compose available")
			}
		}

		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			fmt.Println("  [OK]   docker socket: /var/run/docker.sock exists")
		} else {
			fmt.Printf("  [WARN] docker socket: %v\n", err)
		}

		// Paths
		cfg := config.Default()
		fmt.Println("\n[PATHS]")
		paths := []string{cfg.ConfigDir, cfg.StackDir, cfg.BackupDir, cfg.StateDir}
		for _, p := range paths {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				fmt.Printf("  [OK]   %s\n", p)
			} else {
				fmt.Printf("  [WARN] %s (missing or not a directory)\n", p)
			}
		}

		if _, err := os.Stat(cfg.StackCompose()); err == nil {
			fmt.Printf("  [OK]   stack compose: %s\n", cfg.StackCompose())
		} else {
			fmt.Printf("  [WARN] stack compose not found: %s\n", cfg.StackCompose())
		}

		// Managed stack
		fmt.Println("\n[STACK]")
		if dc != nil {
			managed, err := discovery.ManagedStack(ctx)
			if err != nil {
				fmt.Printf("  [FAIL] list managed: %v\n", err)
			} else if len(managed) == 0 {
				fmt.Println("  [WARN] no Orvyn managed containers found")
			} else {
				running := 0
				for _, ct := range managed {
					if ct.State == "running" {
						running++
					}
				}
				fmt.Printf("  [OK]   %d managed containers (%d running)\n", len(managed), running)
				for _, ct := range managed {
					name := ct.Names[0]
					if len(name) > 0 && name[0] == '/' {
						name = name[1:]
					}
					svc := ct.Labels["com.orvyn.service"]
					if svc == "" {
						svc = "-"
					}
					fmt.Printf("        - %s [%s] %s\n", name, svc, ct.State)
				}
			}
		}

		// Endpoints
		fmt.Println("\n[ENDPOINTS]")
		client := &http.Client{Timeout: 3 * time.Second}
		check := func(url string) {
			resp, err := client.Get(url)
			if err != nil {
				fmt.Printf("  [FAIL] %s: %v\n", url, err)
				return
			}
			resp.Body.Close()
			fmt.Printf("  [OK]   %s -> %d\n", url, resp.StatusCode)
		}
		check("http://127.0.0.1:9090/-/ready")
		check("http://127.0.0.1:3100/ready")

		fmt.Println("\n=== Done ===")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
