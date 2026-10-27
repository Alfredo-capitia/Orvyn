package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [service]",
	Short: "Start Orvyn stack or a specific service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		if len(args) == 0 {
			fmt.Println("[START] Starting Orvyn stack...")
			out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "start")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			fmt.Println("[START] Done")
			return nil
		}

		svc := args[0]
		fmt.Printf("[START] Starting service %s...\n", svc)
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "start", svc)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[START] Done")
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [service]",
	Short: "Stop Orvyn stack or a specific service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		if len(args) == 0 {
			fmt.Println("[STOP] Stopping Orvyn stack...")
			out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "stop")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			fmt.Println("[STOP] Done")
			return nil
		}

		svc := args[0]
		fmt.Printf("[STOP] Stopping service %s...\n", svc)
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "stop", svc)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[STOP] Done")
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [service]",
	Short: "Restart Orvyn stack or a specific service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		if len(args) == 0 {
			fmt.Println("[RESTART] Restarting Orvyn stack...")
			out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "restart")
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			fmt.Println("[RESTART] Done")
			return nil
		}

		svc := args[0]
		fmt.Printf("[RESTART] Restarting service %s...\n", svc)
		out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "restart", svc)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[RESTART] Done")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
}
