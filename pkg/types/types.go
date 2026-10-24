package types

import "time"

type Component struct {
	Name      string
	Type      string // docker, prometheus, loki, alloy, grafana, node-exporter, cadvisor
	Managed   bool   // true if managed by orvyn
	Running   bool
	Image     string
	Container string
}

type Plan struct {
	CreatedAt    time.Time
	Actions      []Action
	Backups      []Backup
	RequiresSudo bool
}

type Action struct {
	Type        string // create_dir, write_file, backup_file, docker_compose_up
	Description string
	Path        string
	Impact      string
}

type Backup struct {
	Original string
	Backup   string
	Reason   string
}

type State struct {
	ManagedComponents []Component
	StackDir          string
	LastPlanHash      string
	InstalledAt       time.Time
}
