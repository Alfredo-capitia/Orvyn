package stack

import (
	"os"
	"path/filepath"

	"github.com/fredy-arias/orvyn/internal/config"
	"github.com/fredy-arias/orvyn/internal/fsutil"
)

func ComposeContent() string {
	return `version: '3.8'

networks:
  orvyn-net:
    driver: bridge

volumes:
  prometheus_data:
  loki_data:
  grafana_data:

services:
  node-exporter:
    image: prom/node-exporter:latest
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

  cadvisor:
    image: gcr.io/cadvisor/cadvisor:latest
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

  prometheus:
    image: prom/prometheus:latest
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

  loki:
    image: grafana/loki:latest
    container_name: orvyn-loki
    restart: unless-stopped
    networks:
      - orvyn-net
    ports:
      - "127.0.0.1:3100:3100"
    volumes:
      - ./loki/loki-config.yml:/etc/loki/local-config.yaml:ro
      - loki_data:/loki
    command: -config.file=/etc/loki/local-config.yaml

  alloy:
    image: grafana/alloy:latest
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
`
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

common:
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

ruler:
  alertmanager_url: http://localhost:9093
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

	if err := fsutil.AtomicWrite(cfg.StackCompose(), []byte(ComposeContent()), 0640); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "prometheus", "prometheus.yml"), []byte(PrometheusConfig()), 0640); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "loki", "loki-config.yml"), []byte(LokiConfig()), 0640); err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(filepath.Join(cfg.StackDir, "alloy", "config.alloy"), []byte(AlloyConfig()), 0640); err != nil {
		return err
	}

	return nil
}
