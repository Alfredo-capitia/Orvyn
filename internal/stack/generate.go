package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alfredo-capitia/Orvyn/internal/config"
	"github.com/Alfredo-capitia/Orvyn/internal/fsutil"
	"github.com/Alfredo-capitia/Orvyn/internal/resources"
)

// projectPlaceholder is replaced with the pinned Compose project name. The name
// is validated before it reaches this function, so it cannot inject YAML.
const projectPlaceholder = "__ORVYN_PROJECT__"

// ComposeContent renders the stack compose file for the given project. The
// top-level "name:" pins the Compose project so that containers and volumes
// are never shared with another directory that happens to use the same base
// name.
func ComposeContent(project string) string {
	l := resources.Detect()
	return strings.ReplaceAll(fmt.Sprintf(`name: "__ORVYN_PROJECT__"

networks:
  orvyn-net:
    driver: bridge
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"

volumes:
  prometheus_data:
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
  loki_data:
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
  grafana_data:
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"

services:
  node-exporter:
    image: prom/node-exporter:v1.8.2
    container_name: orvyn-node-exporter
    restart: unless-stopped
    networks:
      - orvyn-net
    pid: host
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    command:
      - '--path.procfs=/host/proc'
      - '--path.sysfs=/host/sys'
      - '--path.rootfs=/rootfs'
    mem_limit: %s
    cpus: '%s'
    pids_limit: %d
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
      com.orvyn.service: "node-exporter"

  cadvisor:
    image: gcr.io/cadvisor/cadvisor:v0.50.0
    container_name: orvyn-cadvisor
    restart: unless-stopped
    networks:
      - orvyn-net
    volumes:
      - /:/rootfs:ro
      - /var/run:/var/run:rw
      - /sys:/sys:ro
      - /var/lib/docker/:/var/lib/docker:ro
    privileged: false
    mem_limit: %s
    cpus: '%s'
    pids_limit: %d
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
      com.orvyn.service: "cadvisor"

  prometheus:
    image: prom/prometheus:v2.55.1
    container_name: orvyn-prometheus
    restart: unless-stopped
    networks:
      - orvyn-net
    ports:
      - "127.0.0.1:9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    mem_limit: %s
    cpus: '%s'
    pids_limit: %d
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
      com.orvyn.service: "prometheus"

  loki:
    image: grafana/loki:3.3.2
    container_name: orvyn-loki
    restart: unless-stopped
    networks:
      - orvyn-net
    ports:
      - "127.0.0.1:3100:3100"
    volumes:
      - ./loki/loki-config.yml:/etc/loki/local-config.yaml:ro
      - loki_data:/loki
    command: -config.file=/etc/loki/local-config.yaml -validation.allow-structured-metadata=false
    mem_limit: %s
    cpus: '%s'
    pids_limit: %d
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
      com.orvyn.service: "loki"

  alloy:
    image: grafana/alloy:v1.21.3
    container_name: orvyn-alloy
    restart: unless-stopped
    networks:
      - orvyn-net
    volumes:
      - ./alloy/config.alloy:/etc/alloy/config.alloy:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    command:
      - run
      - --server.http.listen-addr=127.0.0.1:12345
      - --storage.path=/var/lib/alloy/data
      - /etc/alloy/config.alloy
    mem_limit: %s
    cpus: '%s'
    pids_limit: %d
    labels:
      com.orvyn.managed: "true"
      com.orvyn.project: "__ORVYN_PROJECT__"
      com.orvyn.service: "alloy"
`, l.NodeExp, l.CPULimit, l.PIDsLimit, l.CAdvisor, l.CPULimit, l.PIDsLimit, l.Prometheus, l.CPULimit, l.PIDsLimit, l.Loki, l.CPULimit, l.PIDsLimit, l.Alloy, l.CPULimit, l.PIDsLimit), projectPlaceholder, project)
}

func PrometheusConfig() string {
	return `global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']

  - job_name: 'cadvisor'
    static_configs:
      - targets: ['cadvisor:8080']
`
}

func LokiConfig() string {
	return `auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  instance_addr: 127.0.0.1
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

query_range:
  results_cache:
    cache:
      embedded_cache:
        enabled: true
        max_size_mb: 100

schema_config:
  configs:
    - from: 2020-10-24
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h

ruler:
  alertmanager_url: http://localhost:9093

limits_config:
  allow_structured_metadata: false
`
}

func AlloyConfig() string {
	return `discovery.docker "containers" {
  host = "unix:///var/run/docker.sock"
}

discovery.relabel "containers" {
  targets = discovery.docker.containers.targets

  rule {
    source_labels = ["__meta_docker_container_name"]
    regex         = "/(.*)"
    target_label  = "container"
  }
}

loki.source.docker "default" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.docker.containers.targets
  labels     = {"job" = "docker"}
  forward_to = [loki.write.local.receiver]
}

loki.write "local" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push"
  }
}
`
}

func GenerateAll() error {
	cfg := config.Default()
	if err := config.EnsureDirs(cfg); err != nil {
		return err
	}

	// Resolve the project before writing: an existing compose file keeps its
	// pinned name (or its legacy directory-derived one) so regenerating the
	// stack never moves it to a different Compose project.
	project := ProjectName(cfg)

	dirs := []string{
		filepath.Join(cfg.StackDir, "prometheus"),
		filepath.Join(cfg.StackDir, "loki"),
		filepath.Join(cfg.StackDir, "alloy"),
		filepath.Join(cfg.StackDir, "grafana", "provisioning"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0750); err != nil {
			return err
		}
	}

	if err := fsutil.AtomicWrite(cfg.StackCompose(), []byte(ComposeContent(project)), 0644); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "prometheus", "prometheus.yml"), []byte(PrometheusConfig()), 0644); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "loki", "loki-config.yml"), []byte(LokiConfig()), 0644); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "alloy", "config.alloy"), []byte(AlloyConfig()), 0644); err != nil {
		return err
	}

	return nil
}
