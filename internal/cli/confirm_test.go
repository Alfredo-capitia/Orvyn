package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmDestructive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes", "y\n", true},
		{"yes upper", "Y\n", true},
		{"yes padded", "  yes  \n", false},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"eof", "", false},
		{"garbage", "sure\n", false},
	}

	lines := []string{"Stack: /tmp/stack/docker-compose.yml", "Volumes will be REMOVED."}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := confirmDestructive(strings.NewReader(tt.input), &out, "ORVYN DOWN", lines)
			if err != nil {
				t.Fatalf("confirmDestructive: %v", err)
			}
			if got != tt.want {
				t.Errorf("confirmDestructive(%q) = %v, want %v", tt.input, got, tt.want)
			}
			prompt := out.String()
			if !strings.Contains(prompt, "ORVYN DOWN") {
				t.Errorf("prompt does not name the operation: %q", prompt)
			}
			for _, line := range lines {
				if !strings.Contains(prompt, line) {
					t.Errorf("prompt does not include %q: %q", line, prompt)
				}
			}
			if !strings.Contains(prompt, "Type 'y' to confirm") {
				t.Errorf("prompt has no confirmation instruction: %q", prompt)
			}
		})
	}
}

func TestDestructiveCommandsAcceptYes(t *testing.T) {
	for _, name := range []string{"down", "rm", "prune", "upgrade"} {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("find %q: %v", name, err)
		}
		if cmd.Flags().Lookup("yes") == nil {
			t.Errorf("command %q does not expose --yes", name)
		}
	}
}
