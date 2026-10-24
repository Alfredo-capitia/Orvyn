# Orvyn

**Automate observability setup on your Linux VPS. Local-first, plan-before-apply, privacy by default.**

Orvyn is a tool that helps you go from "app running on a VPS" to "functional observability" without manually installing, configuring and integrating Prometheus, Grafana, Loki, Alloy and exporters.

## Philosophy

- **Local-first** - Runs on your VPS, no cloud account required
- **Plan-first** - Shows exactly what will be changed before applying
- **Least privilege** - Runs as your user by default, minimal permissions
- **Privacy by default** - Redacts secrets from logs, minimizes data collection
- **Docker API first** - Prefers Docker SDK over shell commands
- **Stack isolated** - Creates observability stack in `/opt/orvyn/stack/`, doesn't touch existing configs

## Features

- **Discovery** - Detects OS, Docker, containers, existing observability stack
- **Plan & Apply** - Safe, confirmed changes with automatic backups
- **Logs in terminal** - `orvyn logs <service> --follow` with automatic redaction
- **Status & Validation** - Check health of your observability stack

## Quick Start

```bash
# Build
go build ./cmd/orvyn

# Discover environment (read-only)
./orvyn discover

# Plan changes (read-only)
./orvyn plan

# Apply with confirmation
./orvyn apply

# Follow logs
./orvyn logs my-app --follow
```

## Security

See [SECURITY.md](SECURITY.md) for detailed security policy.

Key points:
- All logs are treated as potentially sensitive and redacted
- Path traversal protection on all file operations
- No shell command construction from user input
- Audit log of security-relevant events
- Backups before any modifications
- Explicit confirmation required for destructive operations

## Requirements

- Linux
- Docker and Docker Compose
- Go 1.21+ (for building)
- Access to Docker socket (or root)

## Project Structure

See [PHASES.md](PHASES.md) for development roadmap.

## License

TBD
