package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/executil"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Orvyn stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		fmt.Println("[START] Starting Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "start")
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[START] Done")
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Orvyn stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		fmt.Println("[STOP] Stopping Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 120*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "stop")
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		fmt.Println("[STOP] Done")
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart Orvyn stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		fmt.Println("[RESTART] Restarting Orvyn stack...")
		out, err := executil.RunWithTimeout(ctx, 180*time.Second, "docker", "compose", "-f", cfg.StackCompose(), "restart")
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
