package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNoDuplicateCommands guards against a command being registered twice.
// Cobra silently accepts duplicates, which makes the command appear multiple
// times in `orvyn --help` and can shadow flags registered by the other copy.
func TestNoDuplicateCommands(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		name := c.Name()
		if seen[name] {
			t.Errorf("command %q is registered more than once", name)
		}
		seen[name] = true
	}
}

// TestCommandsAreRunnable ensures every registered command has an action,
// so a stub or a forgotten RunE cannot ship as a working subcommand.
func TestCommandsAreRunnable(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		if c.Run == nil && c.RunE == nil {
			t.Errorf("command %q has neither Run nor RunE", c.Name())
		}
	}
}

// TestStatusAndValidateAreImplemented ensures the placeholder bootstrap
// handlers for status and validate were replaced by their implementations.
func TestStatusAndValidateAreImplemented(t *testing.T) {
	for _, c := range []*cobra.Command{statusCmd, validateCmd} {
		if c.RunE == nil {
			t.Errorf("command %q has no RunE implementation", c.Name())
		}
		if c.Run != nil {
			t.Errorf("command %q still has a stub Run handler", c.Name())
		}
	}
}
