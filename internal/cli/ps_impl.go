package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Alfredo-capitia/Orvyn/internal/discovery"
	"github.com/spf13/cobra"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List Orvyn managed containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		containers, err := discovery.ManagedStack(ctx)
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			fmt.Println("No Orvyn managed containers found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSERVICE\tIMAGE\tSTATUS\tSTATE")
		for _, ct := range containers {
			name := ct.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			svc := ct.Labels["com.orvyn.service"]
			if svc == "" {
				svc = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, svc, ct.Image, ct.Status, ct.State)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(psCmd)
}
