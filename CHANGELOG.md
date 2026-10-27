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
- `orvyn backup` to archive the stack and configuration into timestamped `.tar.gz` files, and `orvyn restore` to roll back after a failed change (`--latest`, `--yes`)
- `apply` now takes an automatic backup before it overwrites any stack or config file
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
- Restore is fail-closed: the archive is validated and unpacked into a temporary directory before any file is moved into place; absolute paths, `..`, symlink entries, unknown top-level roots and entries above the size limit are rejected, and the decompressed stream is bounded
- Restore never follows a symlink below the destination root, and cross-filesystem copies land in a sibling temporary file that is renamed into place
- Backups are written `0600` and restore never applies group/other-writable or setuid bits from an archive

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
- Backups are not encrypted at rest: archives are `0600` and rely on filesystem permissions
- Restore overwrites files present in the archive; it does not delete files created after the backup
