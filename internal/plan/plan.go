package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/internal/docker"
)

type Action struct {
	Type   string `json:"type"`
	Desc   string `json:"desc"`
	Path   string `json:"path,omitempty"`
	Impact string `json:"impact"`
}

type Plan struct {
	CreatedAt       string   `json:"created_at"`
	StackDir        string   `json:"stack_dir"`
	BackupDir       string   `json:"backup_dir"`
	Actions         []Action `json:"actions"`
	BackupsPlanned  []string `json:"backups_planned"`
	Notes           []string `json:"notes"`
	ReadOnlyMessage string   `json:"read_only_message"`
}

func Build(ctx context.Context) (*Plan, error) {
	cfg := config.Default()

	p := &Plan{
		CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		StackDir:       cfg.StackDir,
		BackupDir:      cfg.BackupDir,
		Actions:        []Action{},
		BackupsPlanned: []string{"- -> " + filepath.Join(cfg.BackupDir, "backup-<timestamp>.tar.gz") + " (Safety backup before any modifications)"},
		Notes: []string{
			"Stack will be ISOLATED in stack directory/ (won't touch existing docker-compose.yml)",
			"No changes made yet",
			"Use 'orvyn apply' to execute (requires confirmation)",
		},
		ReadOnlyMessage: "[read-only] No changes made.",
	}

	composePath := cfg.StackCompose()
	if _, err := os.Stat(composePath); err == nil {
		c, err := docker.New()
		if err == nil {
			defer c.Close()
			if c.Ping(ctx) == nil {
				if managed, err := discovery.ManagedStack(ctx); err == nil && len(managed) > 0 {
					running := 0
					for _, ct := range managed {
						if ct.State == "running" {
							running++
						}
					}
					if running == len(managed) {
						p.Actions = append(p.Actions, Action{
							Type:   "ok",
							Desc:   fmt.Sprintf("Stack already deployed: %d/%d managed containers running", running, len(managed)),
							Impact: "low",
						})
						p.Actions = append(p.Actions, Action{
							Type:   "info",
							Desc:   "No changes required. Use 'orvyn status' or 'orvyn validate' to inspect.",
							Impact: "low",
						})
						return p, nil
					}
					p.Actions = append(p.Actions, Action{
						Type:   "info",
						Desc:   fmt.Sprintf("Stack files exist but only %d/%d managed containers running", running, len(managed)),
						Impact: "medium",
					})
					p.Actions = append(p.Actions, Action{
						Type:   "docker_compose_up",
						Desc:   "Reconcile observability stack: docker compose up -d",
						Path:   composePath,
						Impact: "high",
					})
					return p, nil
				}
			}
		}
		p.Actions = append(p.Actions, Action{
			Type:   "info",
			Desc:   "Stack files already present but no Orvyn managed containers detected",
			Impact: "medium",
		})
		p.Actions = append(p.Actions, Action{
			Type:   "docker_compose_up",
			Desc:   "Start observability stack: docker compose up -d",
			Path:   composePath,
			Impact: "high",
		})
		return p, nil
	}

	p.Actions = append(p.Actions, Action{
		Type:   "info",
		Desc:   "Existing observability components detected - will attempt to integrate where compatible",
		Impact: "low",
	})
	p.Actions = append(p.Actions, Action{Type: "create_dir", Desc: "Create isolated stack directory", Path: cfg.StackDir, Impact: "low"})
	p.Actions = append(p.Actions, Action{Type: "create_dir", Desc: "Create Prometheus config directory", Path: filepath.Join(cfg.StackDir, "prometheus"), Impact: "low"})
	p.Actions = append(p.Actions, Action{Type: "create_dir", Desc: "Create Loki config directory", Path: filepath.Join(cfg.StackDir, "loki"), Impact: "low"})
	p.Actions = append(p.Actions, Action{Type: "create_dir", Desc: "Create Alloy config directory", Path: filepath.Join(cfg.StackDir, "alloy"), Impact: "low"})
	p.Actions = append(p.Actions, Action{Type: "create_dir", Desc: "Create Grafana provisioning directories", Path: filepath.Join(cfg.StackDir, "grafana", "provisioning"), Impact: "low"})
	p.Actions = append(p.Actions, Action{Type: "write_file", Desc: "Generate docker-compose.yml (node-exporter, cadvisor, Prometheus, Loki, Alloy)", Path: composePath, Impact: "medium"})
	p.Actions = append(p.Actions, Action{Type: "write_file", Desc: "Generate Prometheus configuration with scrape targets", Path: filepath.Join(cfg.StackDir, "prometheus", "prometheus.yml"), Impact: "medium"})
	p.Actions = append(p.Actions, Action{Type: "write_file", Desc: "Generate Loki configuration", Path: filepath.Join(cfg.StackDir, "loki", "loki-config.yml"), Impact: "medium"})
	p.Actions = append(p.Actions, Action{Type: "write_file", Desc: "Generate Alloy configuration for log collection", Path: filepath.Join(cfg.StackDir, "alloy", "config.alloy"), Impact: "medium"})
	p.Actions = append(p.Actions, Action{Type: "docker_compose_up", Desc: "Start observability stack: docker compose up -d", Path: composePath, Impact: "high"})

	return p, nil
}
