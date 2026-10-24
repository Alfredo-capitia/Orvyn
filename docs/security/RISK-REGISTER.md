# Risk Register

| ID | Risk | Severity | Likelihood | Mitigation | Status | Owner |
|---|---|---|---|---|---|---|
| R-001 | Path traversal | High | Medium | fsutil.SafeJoin, symlink checks | In Progress | Core |
| R-002 | Secret leakage in logs | High | Medium | redaction always on | In Progress | Core |
| R-003 | Command injection | High | Low | Avoid shell, executil allowlist | Planned | Core |
| R-004 | Docker socket abuse | High | Medium | Least privilege, read-only first | Planned | Core |
| R-005 | Unsafe file writes | High | Medium | Atomic write, backups, approval | Planned | Core |
| R-006 | Supply chain | Medium-High | Medium | go.sum, govulncheck, dependabot | Planned | Core |
