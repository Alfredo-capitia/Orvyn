# Risk Register

Last reviewed: 2026-10-31. Status values: `Implemented`, `Mitigated` (control in place, residual risk accepted) or `Planned`.

| ID | Risk | Severity | Likelihood | Mitigation | Status | Owner |
|---|---|---|---|---|---|---|
| R-001 | Path traversal | High | Medium | `fsutil.SafeJoin` for generated/extracted paths, symlink checks in restore, `..`/absolute-entry rejection | Implemented | Core |
| R-002 | Secret leakage in logs | High | Medium | Redaction applied to CLI log streams and to the TUI viewport; known limits documented in `docs/THREAT-MODEL.md#secret-redaction-limits` | Implemented (best-effort) | Core |
| R-003 | Command injection | High | Low | No shell; every external command goes through the `executil` allowlist with separate arguments | Implemented | Core |
| R-004 | Docker socket abuse | High | Medium | The socket is mounted read-only and only into Alloy; containers are restricted to managed labels, a dedicated network and host-aware limits. The CLI itself needs the socket, so the residual risk stays with the operator | Mitigated (accepted) | Core |
| R-005 | Unsafe file writes | High | Medium | `fsutil.AtomicWrite`, automatic backup before `apply`, confirmation for destructive commands, fail-closed restore | Implemented | Core |
| R-006 | Supply chain | Medium-High | Medium | `go.sum`, `govulncheck` with reviewed exclusions, Dependabot, pinned images, per-image Trivy baselines, pinned `staticcheck` | Implemented | Core |
