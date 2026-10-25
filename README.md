# Orvyn

[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker%20Compose-v2-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)
[![Linux](https://img.shields.io/badge/Platform-Linux-FCC624?logo=linux&logoColor=black)](https://www.kernel.org/)
[![Local--First](https://img.shields.io/badge/Local--First-Yes-22c55e)](.)

**Local-first CLI to provision, validate and manage an isolated observability stack on Linux VPS with Docker.**

Orvyn takes you from a bare VPS to a working, self-contained observability stack with minimal manual configuration. Everything is generated, isolated and managed by labels with a `plan -> apply` workflow.

## Philosophy

- **Local-first** – Runs entirely on your host. No external SaaS or cloud account required.
- **Plan-before-apply** – Always see what will change before executing anything.
- **Isolated by design** – All files live under Orvyn's own directories. Never touches existing `docker-compose.yml` files.
- **Least privilege** – Designed for non-root usage with user-local paths. Root also supported with system paths.
- **Privacy by default** – Redaction built-in for logs and audit without leaking secrets.
- **Idempotent** – Safe to re-run `up`, `apply` or `plan` without corrupting state.
- **Docker-first** – Prefers the Docker API. Shell usage is strictly allowlisted.

## Stack Components

| Service | Role | Notes |
| --- | --- | --- |
| [`prometheus`](https://prometheus.io/) | Metrics collection & storage | Scrapes `node-exporter`, `cadvisor` and itself. Bound to `127.0.0.1:9090` |
| [`loki`](https://grafana.com/oss/loki/) | Log aggregation | Configured with TSDB schema. Bound to `127.0.0.1:3100` |
| [`alloy`](https://grafana.com/oss/alloy/) | Log/telemetry collector | Collects Docker container logs via `/var/run/docker.sock` and ships to Loki |
| [`node-exporter`](https://github.com/prometheus/node-exporter) | Host metrics | Exposes CPU, memory, disk, network and filesystem metrics |
| [`cadvisor`](https://github.com/google/cadvisor) | Container metrics | Exposes resource usage per running container |

All services run on an isolated bridge network (`orvyn-net`) and are labeled with `com.orvyn.managed=true` and `com.orvyn.service=<name>`.

## Paths

Orvyn automatically selects paths based on UID:

| Context | Config | Stack | Backups | State |
| --- | --- | --- | --- | --- |
| **Non-root** | `~/.config/orvyn/` | `~/.local/share/orvyn/stack/` | `~/.local/share/orvyn/backups/` | `~/.local/state/orvyn/` |
| **Root** | `/etc/orvyn/` | `/opt/orvyn/stack/` | `/opt/orvyn/backups/` | `/var/lib/orvyn/` |

## Installation

### From source

```bash
git clone https://github.com/Alfredo-capitia/Orvyn.git
cd Orvyn
go build ./cmd/orvyn
./orvyn version
```

Optionally move to your `PATH`:

```bash
cp orvyn ~/.local/bin/orvyn  # or /usr/local/bin/orvyn
```

### Requirements

- Linux (tested on modern distros)
- [Docker Engine](https://docs.docker.com/engine/) + [Docker Compose v2](https://docs.docker.com/compose/)
- Access to Docker socket (`/var/run/docker.sock`)
- Go `1.27+` (only for building from source)

## Quick Start

```bash
# See what would be done (read-only)
orvyn plan

# Generate and start the stack with confirmation
orvyn apply

# Check containers and health
orvyn status
orvyn ps

# Validate Docker, stack and endpoints
orvyn validate
orvyn doctor

# View logs
orvyn logs loki --tail 50
orvyn logs alloy -f
```

## CLI Reference

### Discovery & Provisioning

| Command | Description |
| --- | --- |
| `orvyn discover` | Scan host for OS, Docker, existing containers and potential observability stacks (read-only) |
| `orvyn plan` | Generate a state-aware execution plan. Shows `ok` if stack is already healthy with no changes needed |
| `orvyn apply` | Run preflight checks, show plan, request confirmation (`y`) and provision the isolated stack |
| `orvyn up` | Idempotently start/reconcile the stack. Generates files if missing |

### Lifecycle

| Command | Description |
| --- | --- |
| `orvyn start [service]` | Start entire stack or a single service (e.g. `orvyn start loki`) |
| `orvyn stop [service]` | Stop entire stack or a single service |
| `orvyn restart [service]` | Restart entire stack or a single service |
| `orvyn ps` | List only Orvyn-managed containers with service, image, status and state |
| `orvyn down` | Stop and remove containers/network. Volumes preserved by default (`--volumes` to remove) |
| `orvyn rm` | Alias for `down` |
| `orvyn uninstall` | Full uninstall with confirmation. Supports `--force` and `--volumes` |

### Observability & Debugging

| Command | Description |
| --- | --- |
| `orvyn status` | Show managed containers with service labels and health (via `docker inspect`) |
| `orvyn logs <service\|container>` | Stream logs. Supports `-f/--follow`, `-n/--tail`, `-t/--timestamps` |
| `orvyn validate` | Validate Docker daemon, managed stack and local endpoints (`:9090/-/ready`, `:3100/ready`) |
| `orvyn doctor` | Comprehensive diagnostics: Docker, compose, paths, managed stack, endpoints |
| `orvyn info` | Quick overview of Orvyn paths and managed container count |
| `orvyn config` | Show resolved configuration paths for current user/root context |

### Maintenance & Utilities

| Command | Description |
| --- | --- |
| `orvyn exec <service> [cmd...]` | Run a command in a managed container (defaults to `sh`) |
| `orvyn upgrade` | `docker compose pull` + `docker compose up -d` to update images |
| `orvyn prune` | Prune unused Docker images |
| `orvyn completion <bash\|zsh\|fish\|powershell>` | Generate shell autocompletion |
| `orvyn version` | Print Orvyn version |

## Safety & Hardening

- **Label-based control** – Only touches containers with `com.orvyn.managed=true`. Existing workloads are never touched.
- **Preflight validation** – Checks Docker daemon, `docker compose`, socket access, registry reachability and disk space.
- **Apply session tracking** – Tracks partial failures with recovery guidance.
- **Idempotency** – Detects existing managed containers and reconciles safely.
- **Host-aware limits** – Dynamically applies `mem_limit`, `cpus`, `pids_limit` based on `/proc/meminfo` and CPU count.
- **Atomic writes & safe joins** – Uses `fsutil.AtomicWrite` and `fsutil.SafeJoin` to prevent path traversal.
- **Redaction & audit** – Sensitive data is redacted; security-relevant actions are logged.
- **Local-only bindings** – Prometheus/Loki are bound to `127.0.0.1`. Only expose via reverse proxy if needed.
- **Isolated network/volumes** – Dedicated `orvyn-net` and named volumes, all labeled as Orvyn-managed.

## Development

```bash
# Format & build
gofmt -w ./...
go build ./...

# Run tests (if present)
go test ./...
```

## License

See [LICENSE](LICENSE) for details.
