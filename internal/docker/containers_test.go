package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func summary(id string, managed bool, names []string, service string) container.Summary {
	labels := map[string]string{}
	if managed {
		labels["com.orvyn.managed"] = "true"
	}
	if service != "" {
		labels["com.docker.compose.service"] = service
	}
	return container.Summary{ID: id, Names: names, Labels: labels}
}

func TestFindManagedContainer(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"

	tests := []struct {
		name  string
		list  []container.Summary
		query string
		want  string // container ID, "" for nil
	}{
		{
			name: "ignores unmanaged container with the same name",
			list: []container.Summary{
				summary(id, false, []string{"/orvyn-loki"}, "loki"),
			},
			query: "loki",
			want:  "",
		},
		{
			name: "matches a managed container by name",
			list: []container.Summary{
				summary(id, true, []string{"/orvyn-loki"}, "loki"),
			},
			query: "loki",
			want:  id,
		},
		{
			name: "matches a managed container by compose service",
			list: []container.Summary{
				summary(id, true, []string{"/orvyn-prometheus"}, "prometheus"),
			},
			query: "prometheus",
			want:  id,
		},
		{
			name: "matches a managed container by short ID",
			list: []container.Summary{
				summary(id, true, []string{"/orvyn-grafana"}, "grafana"),
			},
			query: id[:12],
			want:  id,
		},
		{
			name: "does not panic when names are empty",
			list: []container.Summary{
				summary(id, true, nil, "loki"),
			},
			query: "loki",
			want:  id,
		},
		{
			name: "skips an unmanaged match and returns the managed one",
			list: []container.Summary{
				summary("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false, []string{"/orvyn-loki"}, "loki"),
				summary(id, true, []string{"/orvyn-loki"}, "loki"),
			},
			query: "loki",
			want:  id,
		},
		{
			name: "returns nil when nothing matches",
			list: []container.Summary{
				summary(id, true, []string{"/orvyn-loki"}, "loki"),
			},
			query: "grafana",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findManagedContainer(tt.list, tt.query)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("findManagedContainer returned %q, want nil", got.ID)
				}
				return
			}
			if got == nil {
				t.Fatalf("findManagedContainer returned nil, want %q", tt.want)
			}
			if got.ID != tt.want {
				t.Errorf("findManagedContainer returned %q, want %q", got.ID, tt.want)
			}
		})
	}
}
