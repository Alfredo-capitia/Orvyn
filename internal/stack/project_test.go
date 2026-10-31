package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
)

func TestDefaultProjectName(t *testing.T) {
	a := DefaultProjectName("/home/alice/.local/share/orvyn/stack")
	b := DefaultProjectName("/home/alice/.local/share/orvyn/stack")
	if a != b {
		t.Fatalf("DefaultProjectName is not deterministic: %q != %q", a, b)
	}

	c := DefaultProjectName("/home/bob/.local/share/orvyn/stack")
	if a == c {
		t.Fatalf("stacks with the same base name collided on project %q", a)
	}

	if !strings.HasPrefix(a, ProjectNamePrefix) {
		t.Errorf("DefaultProjectName(%q) = %q, want prefix %q", "/home/alice/.local/share/orvyn/stack", a, ProjectNamePrefix)
	}
	if !isValidProjectName(a) {
		t.Errorf("DefaultProjectName returned invalid Compose name %q", a)
	}
}

func TestComposeProjectName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "name: orvyn-abc123\nservices: {}\n", "orvyn-abc123"},
		{"quoted", "name: \"orvyn-abc123\"\n", "orvyn-abc123"},
		{"single quoted", "name: 'orvyn-abc123'\n", "orvyn-abc123"},
		{"comment", "name: orvyn-abc123 # pinned\n", "orvyn-abc123"},
		{"leading comment", "# name: ignored\nname: orvyn-abc123\n", "orvyn-abc123"},
		{"absent", "services: {}\n", ""},
		{"indented key is not top level", "services:\n  name: orvyn-abc123\n", ""},
		{"invalid characters", "name: \"orvyn abc; rm -rf\"\n", ""},
		{"empty value", "name: \"\"\n", ""},
		{"leading dash", "name: -orvyn\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComposeProjectName([]byte(tt.in)); got != tt.want {
				t.Errorf("ComposeProjectName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProjectName(t *testing.T) {
	t.Run("missing compose uses derived name", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &config.Config{StackDir: dir}
		want := DefaultProjectName(dir)
		if got := ProjectName(cfg); got != want {
			t.Errorf("ProjectName() = %q, want %q", got, want)
		}
	})

	t.Run("pinned name wins", func(t *testing.T) {
		dir := t.TempDir()
		writeCompose(t, dir, "name: orvyn-pinned\nservices: {}\n")
		cfg := &config.Config{StackDir: dir}
		if got := ProjectName(cfg); got != "orvyn-pinned" {
			t.Errorf("ProjectName() = %q, want %q", got, "orvyn-pinned")
		}
	})

	t.Run("legacy compose keeps the directory-derived name", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "stack")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		writeCompose(t, dir, "services: {}\n")
		cfg := &config.Config{StackDir: dir}
		if got := ProjectName(cfg); got != "stack" {
			t.Errorf("ProjectName() = %q, want legacy %q", got, "stack")
		}
	})

	t.Run("invalid pinned name falls back to legacy", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "stack")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		writeCompose(t, dir, "name: \"bad name\"\nservices: {}\n")
		cfg := &config.Config{StackDir: dir}
		if got := ProjectName(cfg); got != "stack" {
			t.Errorf("ProjectName() = %q, want legacy %q", got, "stack")
		}
	})
}

func TestComposeArgs(t *testing.T) {
	dir := t.TempDir()
	writeCompose(t, dir, "name: orvyn-pinned\n")
	cfg := &config.Config{StackDir: dir}

	got := ComposeArgs(cfg, "up", "-d")
	want := []string{"compose", "-p", "orvyn-pinned", "-f", cfg.StackCompose(), "up", "-d"}
	if len(got) != len(want) {
		t.Fatalf("ComposeArgs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ComposeArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestComposeContentPinsProject(t *testing.T) {
	const project = "orvyn-0123abcd"
	content := ComposeContent(project)

	if !strings.Contains(content, `name: "`+project+`"`) {
		t.Errorf("compose content does not pin the project name")
	}
	if n := strings.Count(content, `com.orvyn.project: "`+project+`"`); n != 9 {
		t.Errorf("compose content has %d project labels, want 9", n)
	}
	if strings.Contains(content, projectPlaceholder) {
		t.Errorf("compose content still contains the project placeholder")
	}
}

func writeCompose(t *testing.T, dir, content string) {
	t.Helper()
	cfg := &config.Config{StackDir: dir}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.StackCompose(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
