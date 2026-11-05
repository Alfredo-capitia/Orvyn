package main

import (
	"fmt"
	"os"

	"github.com/Alfredo-capitia/Orvyn/internal/cli"
)

func main() {
	os.Exit(run())
}

// run keeps the exit path testable: main only forwards its result to os.Exit.
func run() int {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}
	return 0
}
