# Security Policy

Orvyn follows strict security principles. See the full security strategy in the project documentation.

## Core Principles

1. **Local-first** - No cloud dependency, no telemetry by default
2. **Least privilege** - Minimal permissions required
3. **Plan-first** - Preview all changes before execution
4. **Privacy by default** - Data minimization, secret redaction
5. **Secure by design** - Defense in depth

## Security Features

- **Secret redaction** - Best-effort, regex-based masking of some token, key and password patterns in streamed logs. It is not a guarantee; see [docs/THREAT-MODEL.md](docs/THREAT-MODEL.md#secret-redaction-limits) for what is and is not covered
- **Path traversal protection** - Restore validates every archive entry and rejects absolute paths, `..`, symlink escapes and unknown roots before writing, using `safeRel`, `withinRoot` and `ensureNoSymlinks`
- **No shell injection** - Docker API used, args passed separately
- **Audit logging** - Security-relevant events logged without secrets
- **Backups** - Automatic backups before modifications
- **Confirmation required** - Interactive confirmation for destructive operations (`--yes`/`--force` for automation), with an audit entry per attempt
- **Single-writer lock** - every command that mutates containers or files (`apply`, `up`, `down`, `prune`, `upgrade`, `restore`, `rm`, `uninstall`, `start`, `stop`, `restart`) holds a non-blocking `flock` on `<state>/orvyn.lock`; a second run fails with the holder's PID and is audited as `result=locked`. The dashboard is read-only and is not locked
- **Read-only by default** - Discovery and plan don't modify anything

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly.

## Security Checklist for Contributors

- [ ] No hardcoded secrets
- [ ] All user input validated
- [ ] File operations stay under a validated root (`safeRel`/`withinRoot`/`ensureNoSymlinks` or equivalent checks)
- [ ] Logs go through redaction
- [ ] Errors don't leak secrets
- [ ] Tests include security cases
- [ ] gosec/staticcheck pass
