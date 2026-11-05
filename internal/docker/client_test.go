package docker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// testClient points the Docker client at an httptest server so the wrappers can
// be exercised without a daemon. The API version is pinned, which disables
// version negotiation (and therefore the extra /_ping request).
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.44"))
	if err != nil {
		t.Fatalf("new docker client: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return &Client{cli: cli}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestListContainersTruncatesID(t *testing.T) {
	longID := strings.Repeat("a", 64)
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("all"); got != "1" {
			t.Errorf("all = %q, want 1", got)
		}
		writeJSON(t, w, []container.Summary{{
			ID:     longID,
			Names:  []string{"/orvyn-loki"},
			Image:  "grafana/loki:3.3.2",
			State:  "running",
			Status: "Up 2 minutes",
			Labels: map[string]string{"com.orvyn.managed": "true"},
		}})
	})

	got, err := c.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d containers, want 1", len(got))
	}
	if got[0].ID != longID[:12] {
		t.Errorf("ID = %q, want the 12-char prefix %q", got[0].ID, longID[:12])
	}
	if got[0].Names[0] != "/orvyn-loki" || got[0].Labels["com.orvyn.managed"] != "true" {
		t.Errorf("container fields not mapped: %+v", got[0])
	}
}

func TestListContainersReturnsDaemonError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	})

	if _, err := c.ListContainers(context.Background(), false); err == nil {
		t.Fatal("ListContainers returned nil error for a 500 response")
	}
}

func TestContainerLogsDefaultsTail(t *testing.T) {
	var query string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = io.WriteString(w, "line\n")
	})

	rc, err := c.ContainerLogs(context.Background(), "abc123", false, "", false)
	if err != nil {
		t.Fatalf("ContainerLogs: %v", err)
	}
	defer rc.Close()

	for _, want := range []string{"stdout=1", "stderr=1", "tail=100"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q missing %q", query, want)
		}
	}
	if strings.Contains(query, "timestamps=1") {
		t.Errorf("timestamps requested when disabled: %q", query)
	}
}

func TestContainerLogsHonoursExplicitOptions(t *testing.T) {
	var query string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = io.WriteString(w, "line\n")
	})

	rc, err := c.ContainerLogs(context.Background(), "abc123", true, "80", true)
	if err != nil {
		t.Fatalf("ContainerLogs: %v", err)
	}
	defer rc.Close()

	for _, want := range []string{"follow=1", "tail=80", "timestamps=1"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q missing %q", query, want)
		}
	}
}

func TestFindContainerByNameOrService(t *testing.T) {
	list := []container.Summary{
		{
			ID:     strings.Repeat("1", 64),
			Names:  []string{"/orvyn-loki"},
			Labels: map[string]string{"com.orvyn.managed": "true", "com.docker.compose.service": "loki"},
		},
		{
			ID:     strings.Repeat("2", 64),
			Names:  []string{"/orvyn-alloy"},
			Labels: map[string]string{"com.orvyn.managed": "true", "com.docker.compose.service": "alloy"},
		},
		{
			ID:     strings.Repeat("3", 64),
			Names:  []string{"/unmanaged"},
			Labels: map[string]string{},
		},
	}

	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, list)
	})

	tests := []struct {
		name string
		want string
	}{
		{"orvyn-loki", strings.Repeat("1", 64)},
		{"/orvyn-alloy", strings.Repeat("2", 64)},
		{"alloy", strings.Repeat("2", 64)},
		{strings.Repeat("1", 12), strings.Repeat("1", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.FindContainerByNameOrService(context.Background(), tt.name)
			if err != nil {
				t.Fatalf("FindContainerByNameOrService: %v", err)
			}
			if got == nil || got.ID != tt.want {
				t.Fatalf("got %+v, want container %q", got, tt.want)
			}
		})
	}

	t.Run("unmanaged container is invisible", func(t *testing.T) {
		got, err := c.FindContainerByNameOrService(context.Background(), "unmanaged")
		if err != nil {
			t.Fatalf("FindContainerByNameOrService: %v", err)
		}
		if got != nil {
			t.Fatalf("resolved an unmanaged container: %+v", got)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		got, err := c.FindContainerByNameOrService(context.Background(), "nope")
		if err != nil {
			t.Fatalf("FindContainerByNameOrService: %v", err)
		}
		if got != nil {
			t.Fatalf("resolved a container for an unknown name: %+v", got)
		}
	})
}

func TestContainerTTY(t *testing.T) {
	t.Run("tty enabled", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, map[string]any{"Id": "abc", "Config": map[string]any{"Tty": true}})
		})
		tty, err := c.ContainerTTY(context.Background(), "abc")
		if err != nil {
			t.Fatalf("ContainerTTY: %v", err)
		}
		if !tty {
			t.Error("TTY reported false for a container created with a TTY")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, map[string]any{"Id": "abc"})
		})
		tty, err := c.ContainerTTY(context.Background(), "abc")
		if err != nil {
			t.Fatalf("ContainerTTY: %v", err)
		}
		if tty {
			t.Error("TTY reported true without a config")
		}
	})

	t.Run("inspect error", func(t *testing.T) {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"no such container"}`, http.StatusNotFound)
		})
		if _, err := c.ContainerTTY(context.Background(), "gone"); err == nil {
			t.Fatal("ContainerTTY returned nil error for a missing container")
		}
	})
}

func TestContainerStatsPassesStreamFlag(t *testing.T) {
	var query string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"read":"2026-01-01T00:00:00Z"}`)
	})

	body, err := c.ContainerStats(context.Background(), "abc", true)
	if err != nil {
		t.Fatalf("ContainerStats: %v", err)
	}
	defer body.Close()
	if !strings.Contains(query, "stream=1") {
		t.Errorf("query %q missing stream=1", query)
	}
}

func TestPingAndInfo(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
		case strings.HasSuffix(r.URL.Path, "/info"):
			writeJSON(t, w, map[string]any{
				"Containers":        3,
				"ContainersRunning": 2,
				"Images":            7,
				"ServerVersion":     "28.5.2",
				"OperatingSystem":   "Ubuntu 24.04",
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info["ContainersRunning"] != 2 || info["ServerVersion"] != "28.5.2" {
		t.Errorf("info not mapped: %+v", info)
	}
}

func TestCloseIsNilSafe(t *testing.T) {
	var nilClient *Client
	if err := nilClient.Close(); err != nil {
		t.Errorf("nil client Close: %v", err)
	}
	if err := (&Client{}).Close(); err != nil {
		t.Errorf("empty client Close: %v", err)
	}
}

func TestNewUsesDockerEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	t.Setenv("DOCKER_API_VERSION", "")

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
