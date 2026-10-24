# Security Policy

VPSMon follows strict security principles. See the full security strategy in the project documentation.

## Core Principles

1. **Local-first** - No cloud dependency, no telemetry by default
2. **Least privilege** - Minimal permissions required
3. **Plan-first** - Preview all changes before execution
4. **Privacy by default** - Data minimization, secret redaction
5. **Secure by design** - Defense in depth

## Security Features

- **Secret redaction** - Automatic redaction of tokens, keys, passwords from logs/output
- **Path traversal protection** - All file paths validated with `fsutil.SafeJoin`
- **No shell injection** - Docker API used, args passed separately
- **Audit logging** - Security-relevant events logged without secrets
- **Backups** - Automatic backups before modifications
- **Confirmation required** - Interactive confirmation for destructive operations
- **Read-only by default** - Discovery and plan don't modify anything

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly.

## Security Checklist for Contributors

- [ ] No hardcoded secrets
- [ ] All user input validated
- [ ] File operations use SafeJoin
- [ ] Logs go through redaction
- [ ] Errors don't leak secrets
- [ ] Tests include security cases
- [ ] gosec/staticcheck pass
