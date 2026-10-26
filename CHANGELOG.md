# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Isolated observability stack (Prometheus, Loki, Alloy, node-exporter, cadvisor) generated with pinned image versions
- `apply` with preflight checks, interactive confirmation, partial-failure tracking and audit logging
- State-aware `plan` that reports `ok` when the stack already matches the desired state
- Lifecycle commands: `up`, `ps`, `start`, `stop`, `restart`, `down`, `rm`, `uninstall`
- Operational commands: `status`, `validate`, `doctor`, `info`, `config`
- Maintenance commands: `prune`, `upgrade`
- `exec` into managed containers and `logs` with `--follow`, `--tail` and `--timestamps`
- Shell completion for bash, zsh, fish and powershell
- Interactive TUI dashboard (`orvyn dashboard`) with Overview, Logs, Metrics, Details and Help views
- Host metrics from `/proc` and per-container stats from the Docker API
- Health inspection via `docker inspect` with a cache to avoid repeated calls
- Incremental filter and quick start/stop/restart actions with confirmation

### Security

- All external commands routed through an allowlisted `executil`
- Destructive operations restricted to containers labeled `com.orvyn.managed=true`
- Prometheus and Loki bound to `127.0.0.1` only
- Redaction of secrets in logs and audit entries
- Atomic writes and path-traversal protection via `fsutil`

### Changed

- Stack images pinned to explicit versions (no `:latest`)
- Generated config files written with `0644` so containers can read them
- Paths are user-local by default, system-wide only when running as root
- CI pinned to Go `1.27.3` with SHA-pinned actions, `concurrency` and job timeouts

### Fixed

- `logs` was registered twice, so it appeared duplicated in `orvyn --help`
- `status` and `validate` still carried placeholder handlers from the initial skeleton
- Built binary `orvyn` was tracked in git and is now ignored

### Known limitations

- `govulncheck` is disabled in CI: GO-2026-4887 and GO-2026-4883 in `docker/docker v28.5.2` have no upstream fix yet
- Backup creation before modifications is not implemented yet, and there is no `restore` command
