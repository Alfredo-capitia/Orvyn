package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useOSRelease points OSRelease at a fixture so the tests do not depend on the
// host distribution.
func useOSRelease(t *testing.T, body string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	old := osReleasePath
	osReleasePath = path
	t.Cleanup(func() { osReleasePath = old })
}

func TestOSRelease(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "pretty name wins and quotes are stripped",
			body: "NAME=Ubuntu\nVERSION=\"24.04.1 LTS\"\nPRETTY_NAME=\"Ubuntu 24.04.1 LTS\"\n",
			want: "Ubuntu 24.04.1 LTS",
		},
		{
			name: "name and version are combined",
			body: "NAME=Debian\nVERSION=\"12 (bookworm)\"\n",
			want: "Debian 12 (bookworm)",
		},
		{
			name: "name alone is enough",
			body: "NAME=Alpine\n",
			want: "Alpine",
		},
		{
			name: "unrecognised content is unknown",
			body: "ID=linux\n",
			want: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			useOSRelease(t, tc.body)
			if got := OSRelease(); got != tc.want {
				t.Errorf("OSRelease() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOSReleaseMissingFile(t *testing.T) {
	old := osReleasePath
	osReleasePath = filepath.Join(t.TempDir(), "does-not-exist")
	t.Cleanup(func() { osReleasePath = old })

	if got := OSRelease(); got != "unknown" {
		t.Errorf("OSRelease() = %q, want unknown", got)
	}
}

// useDaemon points the Docker client at an httptest server and clears the TLS
// variables so the client talks plain HTTP.
func useDaemon(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+srv.Listener.Addr().String())
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")
}

func containersHandler(containers []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
		case strings.HasSuffix(r.URL.Path, "/info"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ServerVersion": "27.0.3",
				"Containers":    4,
			})
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(containers)
		default:
			http.NotFound(w, r)
		}
	}
}

func TestRunWithDaemon(t *testing.T) {
	useOSRelease(t, "PRETTY_NAME=\"Test Linux\"\n")
	useDaemon(t, containersHandler([]map[string]any{
		{"Id": "aaaaaaaaaaaaaaaaaaaa", "Names": []string{"/orvyn-prometheus"}, "Image": "prom/prometheus:v2.55.1", "State": "running"},
		{"Id": "bbbbbbbbbbbbbbbbbbbb", "Names": []string{"/orvyn-grafana"}, "Image": "grafana/grafana:11.0.0", "State": "running"},
		{"Id": "cccccccccccccccccccc", "Names": []string{"/orvyn-loki"}, "Image": "loki/loki:3.3.2", "State": "running"},
		{"Id": "dddddddddddddddddddd", "Names": []string{"/unrelated"}, "Image": "busybox:1.36", "State": "running"},
		// A malformed response with no names must be skipped, not panic.
		{"Id": "eeeeeeeeeeeeeeeeeeee", "Image": "grafana/alloy:v1.21.3", "State": "running"},
	}))

	res, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DockerAvailable {
		t.Fatal("DockerAvailable = false, want true")
	}
	if got := res.DockerInfo["ServerVersion"]; got != "27.0.3" {
		t.Errorf("DockerInfo[ServerVersion] = %v, want 27.0.3", got)
	}
	if len(res.Containers) != 5 {
		t.Errorf("Containers = %d, want 5", len(res.Containers))
	}
	want := []string{"prometheus (/orvyn-prometheus)", "grafana (/orvyn-grafana)", "loki (/orvyn-loki)"}
	if len(res.ExistingStack) != len(want) {
		t.Fatalf("ExistingStack = %v, want %v", res.ExistingStack, want)
	}
	for i := range want {
		if res.ExistingStack[i] != want[i] {
			t.Errorf("ExistingStack[%d] = %q, want %q", i, res.ExistingStack[i], want[i])
		}
	}
}

func TestRunWithoutDocker(t *testing.T) {
	useOSRelease(t, "PRETTY_NAME=\"Test Linux\"\n")
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/orvyn.sock")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")

	res, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DockerAvailable {
		t.Error("DockerAvailable = true, want false")
	}
	if res.OS != "Test Linux" {
		t.Errorf("OS = %q, want Test Linux", res.OS)
	}
	if len(res.Containers) != 0 || len(res.ExistingStack) != 0 {
		t.Errorf("expected no containers, got %+v", res)
	}
}

func TestRunWhenPingFails(t *testing.T) {
	useDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "daemon down", http.StatusInternalServerError)
	}))

	res, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DockerAvailable {
		t.Error("DockerAvailable = true, want false")
	}
}

func TestRunWhenListFails(t *testing.T) {
	useDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
			_, _ = io.WriteString(w, "OK")
		case strings.HasSuffix(r.URL.Path, "/info"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"ServerVersion":"27.0.3"}`)
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	})

	res, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.DockerAvailable {
		t.Error("DockerAvailable = false, want true")
	}
	if len(res.Containers) != 0 {
		t.Errorf("Containers = %v, want empty", res.Containers)
	}
}

func TestRunWhenClientCannotBeCreated(t *testing.T) {
	useOSRelease(t, "PRETTY_NAME=\"Test Linux\"\n")
	t.Setenv("DOCKER_HOST", "invalid://host")

	res, err := Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.DockerAvailable {
		t.Error("DockerAvailable = true, want false")
	}
}

func TestManagedStackFiltersByLabel(t *testing.T) {
	useDaemon(t, containersHandler([]map[string]any{
		{"Id": "aaaaaaaaaaaaaaaaaaaa", "Names": []string{"/one"}, "Labels": map[string]string{"com.orvyn.managed": "true"}},
		{"Id": "bbbbbbbbbbbbbbbbbbbb", "Names": []string{"/two"}, "Labels": map[string]string{"com.orvyn.managed": "true"}},
		{"Id": "cccccccccccccccccccc", "Names": []string{"/three"}, "Labels": map[string]string{"com.orvyn.managed": "false"}},
		{"Id": "dddddddddddddddddddd", "Names": []string{"/four"}, "Labels": map[string]string{"other": "label"}},
	}))

	conts, err := ManagedStack(context.Background())
	if err != nil {
		t.Fatalf("ManagedStack: %v", err)
	}
	if len(conts) != 2 {
		t.Fatalf("ManagedStack returned %d containers, want 2: %+v", len(conts), conts)
	}
	if conts[0].ID != "aaaaaaaaaaaa" || conts[1].ID != "bbbbbbbbbbbb" {
		t.Errorf("unexpected container IDs: %q, %q", conts[0].ID, conts[1].ID)
	}
}

func TestManagedStackErrors(t *testing.T) {
	t.Run("ping failure", func(t *testing.T) {
		useDaemon(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		}))
		if _, err := ManagedStack(context.Background()); err == nil {
			t.Fatal("ManagedStack succeeded with a failing daemon")
		}
	})

	t.Run("list failure", func(t *testing.T) {
		useDaemon(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/_ping") {
				w.Header().Set("API-Version", "1.44")
				_, _ = io.WriteString(w, "OK")
				return
			}
			http.Error(w, "boom", http.StatusInternalServerError)
		})
		if _, err := ManagedStack(context.Background()); err == nil {
			t.Fatal("ManagedStack succeeded with a failing container list")
		}
	})

	t.Run("client creation failure", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "invalid://host")
		if _, err := ManagedStack(context.Background()); err == nil {
			t.Fatal("ManagedStack succeeded with an invalid DOCKER_HOST")
		}
	})
}
