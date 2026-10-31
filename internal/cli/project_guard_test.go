package cli

import (
	"testing"

	"github.com/Alfredo-capitia/Orvyn/internal/docker"
)

func TestForeignProjectContainers(t *testing.T) {
	container := func(id, project string, managed bool) docker.Container {
		labels := map[string]string{}
		if project != "" {
			labels["com.docker.compose.project"] = project
		}
		if managed {
			labels["com.orvyn.managed"] = "true"
		}
		return docker.Container{ID: id, Labels: labels}
	}

	tests := []struct {
		name       string
		containers []docker.Container
		want       []string
	}{
		{
			name:       "no containers",
			containers: nil,
			want:       nil,
		},
		{
			name: "all managed containers in the project",
			containers: []docker.Container{
				container("a", "orvyn-1234", true),
				container("b", "orvyn-1234", true),
			},
			want: nil,
		},
		{
			name: "unmanaged container in the project",
			containers: []docker.Container{
				container("c", "orvyn-1234", false),
			},
			want: []string{"c"},
		},
		{
			name: "managed containers from another project are ignored",
			containers: []docker.Container{
				container("d", "other", true),
				container("e", "", true),
			},
			want: nil,
		},
		{
			name: "mix returns only foreign containers",
			containers: []docker.Container{
				container("f", "orvyn-1234", true),
				container("g", "orvyn-1234", false),
				container("h", "orvyn-1234", false),
			},
			want: []string{"g", "h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := foreignProjectContainers(tt.containers, "orvyn-1234")
			if len(got) != len(tt.want) {
				t.Fatalf("foreignProjectContainers returned %d container(s), want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i].ID != tt.want[i] {
					t.Errorf("container %d = %q, want %q", i, got[i].ID, tt.want[i])
				}
			}
		})
	}
}
