package planner

import (
	"context"
	"time"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/discovery"
	"github.com/fredy-arias/orvyn/pkg/types"
)

func Generate(ctx context.Context) (*types.Plan, error) {
	res, _ := discovery.Run(ctx)

	cfg := config.Default()
	plan := &types.Plan{
		CreatedAt: time.Now(),
		Actions:   []types.Action{},
		Backups:   []types.Backup{},
	}

	// Check if we need to reuse existing stack
	reuse := false
	for _, s := range res.ExistingStack {
		if s != "" {
			reuse = true
			break
		}
	}

	if reuse {
		plan.Actions = append(plan.Actions, types.Action{
			Type:        "info",
			Description: "Existing observability components detected - will attempt to integrate where compatible",
			Path:        "-",
			Impact:      "low",
		})
	}

	plan.Actions = append(plan.Actions,
		types.Action{
			Type:        "create_dir",
			Description: "Create isolated stack directory",
			Path:        cfg.StackDir,
			Impact:      "low",
		},
		types.Action{
			Type:        "create_dir",
			Description: "Create Prometheus config directory",
			Path:        cfg.StackDir + "/prometheus",
			Impact:      "low",
		},
		types.Action{
			Type:        "create_dir",
			Description: "Create Loki config directory",
			Path:        cfg.StackDir + "/loki",
			Impact:      "low",
		},
		types.Action{
			Type:        "create_dir",
			Description: "Create Alloy config directory",
			Path:        cfg.StackDir + "/alloy",
			Impact:      "low",
		},
		types.Action{
			Type:        "create_dir",
			Description: "Create Grafana provisioning directories",
			Path:        cfg.StackDir + "/grafana/provisioning",
			Impact:      "low",
		},
		types.Action{
			Type:        "write_file",
			Description: "Generate docker-compose.yml (node-exporter, cadvisor, Prometheus, Loki, Alloy)",
			Path:        cfg.StackCompose(),
			Impact:      "medium",
		},
		types.Action{
			Type:        "write_file",
			Description: "Generate Prometheus configuration with scrape targets",
			Path:        cfg.StackDir + "/prometheus/prometheus.yml",
			Impact:      "medium",
		},
		types.Action{
			Type:        "write_file",
			Description: "Generate Loki configuration",
			Path:        cfg.StackDir + "/loki/loki-config.yml",
			Impact:      "medium",
		},
		types.Action{
			Type:        "write_file",
			Description: "Generate Alloy configuration for log collection",
			Path:        cfg.StackDir + "/alloy/config.alloy",
			Impact:      "medium",
		},
		types.Action{
			Type:        "docker_compose_up",
			Description: "Start observability stack: docker compose up -d",
			Path:        cfg.StackCompose(),
			Impact:      "high",
		},
	)

	// Backups only if touching existing files
	if reuse {
		plan.Backups = append(plan.Backups, types.Backup{
			Original: "-",
			Backup:   cfg.BackupDir + "/backup-<timestamp>.tar.gz",
			Reason:   "Safety backup before any modifications",
		})
	} else {
		plan.Actions = append(plan.Actions, types.Action{
			Type:        "info",
			Description: "No existing observability stack detected - creating isolated stack",
			Path:        "-",
			Impact:      "low",
		})
	}

	return plan, nil
}
