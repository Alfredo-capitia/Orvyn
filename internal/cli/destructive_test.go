package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/audit"
	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// testEnv prepares an isolated host for a command: a fake HOME (so
// config.Default() writes inside the test), a fake daemon answering the Docker
// client, and a fake "docker" binary that records the Compose arguments it was
// asked to run. No real container, daemon or network is involved.
type testEnv struct {
	home      string
	compose   string
	project   string
	dockerLog string
}

func newTestEnv(t *testing.T, containers []container.Summary, dockerBody string) *testEnv {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USER", "audit-test")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")

	cfg := config.Default()
	if err := config.EnsureDirs(cfg); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	env := &testEnv{
		home:      home,
		compose:   cfg.StackCompose(),
		project:   "orvyn-audittest",
		dockerLog: filepath.Join(home, "docker.log"),
	}
	compose := "name: \"" + env.project + "\"\n\nservices: {}\n"
	if err := os.WriteFile(env.compose, []byte(compose), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	binDir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE_DOCKER_LOG\"\n" + dockerBody
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("FAKE_DOCKER_LOG", env.dockerLog)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(containers); err != nil {
				t.Errorf("encode containers: %v", err)
			}
		default:
			t.Errorf("unexpected daemon request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())

	return env
}

func managedContainer(project, id, service, state string) container.Summary {
	labels := map[string]string{
		"com.orvyn.managed":          "true",
		"com.docker.compose.project": project,
	}
	if service != "" {
		labels["com.docker.compose.service"] = service
		labels["com.orvyn.service"] = service
	}
	name := "/orvyn-" + service
	if service == "" {
		name = "/orvyn-anonymous"
	}
	return container.Summary{
		ID:     id,
		Names:  []string{name},
		Image:  "grafana/stack:latest",
		State:  state,
		Status: "Up 2 minutes",
		Labels: labels,
	}
}

func (e *testEnv) auditEvents(t *testing.T) []audit.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.home, ".local", "state", "orvyn", "audit.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read audit log: %v", err)
	}
	var events []audit.Event
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var ev audit.Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			t.Fatalf("decode audit event %q: %v", scanner.Text(), err)
		}
		events = append(events, ev)
	}
	return events
}

func (e *testEnv) dockerCalls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e.dockerLog)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read docker log: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// withStreams swaps stdin/stdout for the duration of fn. The commands read
// os.Stdin directly, so this is the only way to exercise the prompt and the
// skip flag without a terminal.
func withStreams(t *testing.T, stdin string, fn func() error) (string, error) {
	t.Helper()

	in, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create stdin file: %v", err)
	}
	if _, err := in.WriteString(stdin); err != nil {
		t.Fatalf("seed stdin: %v", err)
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind stdin: %v", err)
	}

	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create stdout file: %v", err)
	}

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	defer func() {
		os.Stdin, os.Stdout = oldIn, oldOut
		in.Close()
		out.Close()
	}()

	runErr := fn()

	if err := out.Close(); err != nil {
		t.Fatalf("close stdout file: %v", err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatalf("read stdout file: %v", err)
	}
	return string(data), runErr
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func runCommand(t *testing.T, cmd *cobra.Command, stdin string) (string, error) {
	t.Helper()
	resetFlags(cmd)
	return withStreams(t, stdin, func() error {
		return cmd.RunE(cmd, nil)
	})
}

func TestDestructiveCommandsAbortWithoutApproval(t *testing.T) {
	commands := []struct {
		name   string
		cmd    *cobra.Command
		header string
		op     string
	}{
		{"down", downCmd, "ORVYN DOWN", "down"},
		{"rm", rmCmd, "ORVYN RM", "rm"},
		{"prune", pruneCmd, "ORVYN PRUNE", "prune"},
		{"upgrade", upgradeCmd, "ORVYN UPGRADE", "upgrade"},
		{"uninstall", uninstallCmd, "ORVYN UNINSTALL", "uninstall"},
	}

	// "n" is an explicit refusal; an empty stdin is EOF, which must abort too.
	for _, input := range []string{"n\n", ""} {
		label := "declined"
		if input == "" {
			label = "eof"
		}
		for _, tc := range commands {
			t.Run(tc.name+"/"+label, func(t *testing.T) {
				env := newTestEnv(t, nil, "echo should-not-run\n")

				out, err := runCommand(t, tc.cmd, input)
				if err != nil {
					t.Fatalf("%s returned an error on %s: %v", tc.name, label, err)
				}
				if !strings.Contains(out, tc.header) {
					t.Errorf("prompt does not name %s:\n%s", tc.header, out)
				}
				if !strings.Contains(out, "Type 'y' to confirm") {
					t.Errorf("prompt has no confirmation instruction:\n%s", out)
				}
				if !strings.Contains(out, "Abort") {
					t.Errorf("aborted command did not say so:\n%s", out)
				}
				if calls := env.dockerCalls(t); len(calls) != 0 {
					t.Errorf("docker was executed after an abort: %v", calls)
				}

				events := env.auditEvents(t)
				if len(events) != 1 {
					t.Fatalf("got %d audit events, want 1: %+v", len(events), events)
				}
				if events[0].Operation != tc.op || events[0].Result != "aborted" {
					t.Errorf("audit event = %+v, want operation %q result aborted", events[0], tc.op)
				}
				if events[0].Approved {
					t.Error("aborted command was audited as approved")
				}
			})
		}
	}
}

func TestDestructiveCommandsRunAfterConfirmation(t *testing.T) {
	project := "orvyn-audittest"
	containers := []container.Summary{
		managedContainer(project, strings.Repeat("a", 64), "loki", "running"),
		managedContainer(project, strings.Repeat("b", 64), "", "exited"),
	}

	tests := []struct {
		name       string
		cmd        *cobra.Command
		input      string
		wantCalls  []string
		wantOutput []string
	}{
		{
			name:       "down",
			cmd:        downCmd,
			input:      "y\n",
			wantCalls:  []string{"compose -p " + project + " -f", " down"},
			wantOutput: []string{"[DOWN] Targeting 2 Orvyn-managed container(s)", "orvyn-loki [loki] running", "orvyn-anonymous [-] exited", "[DOWN] Done"},
		},
		{
			name:       "rm",
			cmd:        rmCmd,
			input:      "Y\n",
			wantCalls:  []string{"compose -p " + project + " -f", " down"},
			wantOutput: []string{"[RM] Targeting 2 Orvyn-managed container(s)", "[RM] Done"},
		},
		{
			name:       "prune",
			cmd:        pruneCmd,
			input:      "y\n",
			wantCalls:  []string{"image prune -f"},
			wantOutput: []string{"[PRUNE] Done"},
		},
		{
			name:       "upgrade",
			cmd:        upgradeCmd,
			input:      "y\n",
			wantCalls:  []string{"compose -p " + project + " -f", " pull", " up -d"},
			wantOutput: []string{"[UPGRADE] Done"},
		},
		{
			name:       "uninstall",
			cmd:        uninstallCmd,
			input:      "y\n",
			wantCalls:  []string{"compose -p " + project + " -f", " down"},
			wantOutput: []string{"[UNINSTALL] Targeting 2 Orvyn-managed container(s)", "[UNINSTALL] Done"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, containers, "printf 'fake-output\\n'\n")

			out, err := runCommand(t, tc.cmd, tc.input)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}

			calls := env.dockerCalls(t)
			if len(calls) == 0 {
				t.Fatal("docker was not executed after confirmation")
			}
			joined := strings.Join(calls, "\n")
			for _, want := range tc.wantCalls {
				if !strings.Contains(joined, want) {
					t.Errorf("docker calls missing %q:\n%s", want, joined)
				}
			}
			for _, want := range tc.wantOutput {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}

			events := env.auditEvents(t)
			if len(events) != 1 {
				t.Fatalf("got %d audit events, want 1: %+v", len(events), events)
			}
			if events[0].Result != "ok" || !events[0].Approved {
				t.Errorf("audit event = %+v, want result ok and approved", events[0])
			}
		})
	}
}

func TestDestructiveCommandsSkipPromptWithYes(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		flag  string
		calls []string
	}{
		{"down", downCmd, "yes", []string{" down"}},
		{"rm", rmCmd, "yes", []string{" down"}},
		{"prune", pruneCmd, "yes", []string{"image prune -f"}},
		{"upgrade", upgradeCmd, "yes", []string{" pull", " up -d"}},
		{"uninstall", uninstallCmd, "force", []string{" down"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, nil, "printf 'fake-output\\n'\n")
			resetFlags(tc.cmd)
			if err := tc.cmd.Flags().Set(tc.flag, "true"); err != nil {
				t.Fatalf("set --%s: %v", tc.flag, err)
			}

			// Empty stdin is EOF: the command must not block or abort when the
			// skip flag is set.
			out, err := withStreams(t, "", func() error { return tc.cmd.RunE(tc.cmd, nil) })
			if err != nil {
				t.Fatalf("%s --%s: %v", tc.name, tc.flag, err)
			}
			if strings.Contains(out, "Type 'y' to confirm") {
				t.Errorf("--%s still prompted:\n%s", tc.flag, out)
			}
			joined := strings.Join(env.dockerCalls(t), "\n")
			for _, want := range tc.calls {
				if !strings.Contains(joined, want) {
					t.Errorf("docker calls missing %q:\n%s", want, joined)
				}
			}
			events := env.auditEvents(t)
			if len(events) != 1 || events[0].Result != "ok" {
				t.Errorf("audit events = %+v, want one ok event", events)
			}
		})
	}
}

func TestDownAndUninstallVolumesFlag(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		flag string
	}{
		{"down", downCmd, "volumes"},
		{"uninstall", uninstallCmd, "volumes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, nil, "printf 'fake-output\\n'\n")
			resetFlags(tc.cmd)
			skip := "yes"
			if tc.cmd == uninstallCmd {
				skip = "force"
			}
			if err := tc.cmd.Flags().Set(skip, "true"); err != nil {
				t.Fatalf("set --%s: %v", skip, err)
			}
			if err := tc.cmd.Flags().Set(tc.flag, "true"); err != nil {
				t.Fatalf("set --%s: %v", tc.flag, err)
			}

			out, err := withStreams(t, "", func() error { return tc.cmd.RunE(tc.cmd, nil) })
			if err != nil {
				t.Fatalf("%s --%s: %v", tc.name, tc.flag, err)
			}
			if tc.cmd == downCmd && !strings.Contains(out, "REMOVED") {
				t.Errorf("volume removal not announced:\n%s", out)
			}
			if calls := strings.Join(env.dockerCalls(t), "\n"); !strings.Contains(calls, " down -v") {
				t.Errorf("compose down did not receive -v:\n%s", calls)
			}
		})
	}
}

func TestDownRefusesForeignContainersInProject(t *testing.T) {
	const project = "orvyn-audittest"
	env := newTestEnv(t, []container.Summary{{
		ID:     strings.Repeat("c", 64),
		Names:  []string{"/someone-elses-db"},
		State:  "running",
		Labels: map[string]string{"com.docker.compose.project": project},
	}}, "printf 'should-not-run\\n'\n")

	resetFlags(downCmd)
	if err := downCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set --yes: %v", err)
	}
	_, err := withStreams(t, "", func() error { return downCmd.RunE(downCmd, nil) })
	if err == nil {
		t.Fatal("down accepted a compose project with a foreign container")
	}
	for _, want := range []string{"refusing to modify compose project", "someone-elses-db", "not Orvyn-managed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if calls := env.dockerCalls(t); len(calls) != 0 {
		t.Errorf("docker was executed despite the ownership guard: %v", calls)
	}
	if events := env.auditEvents(t); len(events) != 0 {
		t.Errorf("guard failure was audited as an executed operation: %+v", events)
	}
}

func TestDownAllowsManagedContainersFromOtherProjects(t *testing.T) {
	project := "orvyn-audittest"
	env := newTestEnv(t, []container.Summary{
		managedContainer(project, strings.Repeat("a", 64), "loki", "running"),
		managedContainer("orvyn-other", strings.Repeat("d", 64), "prometheus", "running"),
	}, "printf 'fake-output\\n'\n")

	resetFlags(downCmd)
	if err := downCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set --yes: %v", err)
	}
	if _, err := withStreams(t, "", func() error { return downCmd.RunE(downCmd, nil) }); err != nil {
		t.Fatalf("down: %v", err)
	}
	if calls := env.dockerCalls(t); len(calls) == 0 {
		t.Fatal("down did not run against the project")
	}
}

func TestDownWithoutComposeFileDoesNothing(t *testing.T) {
	env := newTestEnv(t, nil, "printf 'should-not-run\\n'\n")
	if err := os.Remove(env.compose); err != nil {
		t.Fatalf("remove compose file: %v", err)
	}

	resetFlags(downCmd)
	if err := downCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set --yes: %v", err)
	}
	_, err := withStreams(t, "", func() error { return downCmd.RunE(downCmd, nil) })
	if err == nil {
		t.Fatal("down succeeded without a compose file")
	}
	if !strings.Contains(err.Error(), "nothing to down") {
		t.Errorf("error = %v, want a 'nothing to down' message", err)
	}
	if calls := env.dockerCalls(t); len(calls) != 0 {
		t.Errorf("docker was executed without a compose file: %v", calls)
	}
}

func TestDownAuditsExecutionFailure(t *testing.T) {
	env := newTestEnv(t, nil, "echo 'compose failed' >&2\nexit 9\n")

	resetFlags(downCmd)
	if err := downCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set --yes: %v", err)
	}
	_, err := withStreams(t, "", func() error { return downCmd.RunE(downCmd, nil) })
	if err == nil {
		t.Fatal("down returned nil when docker compose failed")
	}

	events := env.auditEvents(t)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1: %+v", len(events), events)
	}
	if events[0].Result != "failed" {
		t.Errorf("audit event = %+v, want result failed", events[0])
	}
	if events[0].Approved {
		t.Error("failed operation was audited as approved")
	}
}
