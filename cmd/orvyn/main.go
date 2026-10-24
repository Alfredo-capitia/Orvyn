package main

import (
	"fmt"
	"os"

	"github.com/fredy-arias/orvyn/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
