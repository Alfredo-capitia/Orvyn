# Threat Model (STRIDE)

## Assets
- Credentials, tokens, keys
- Application logs
- Infrastructure topology
- Docker daemon access
- State, backups, audit logs
- Binaries and dependencies

## Threats and Mitigations
| Threat | Example | Mitigation |
|---|---|---|
| Spoofing | Fake config | Validate paths/sources |
| Tampering | Modify configs | Backups, atomic writes, approval |
| Repudiation | No trace | Audit log (metadata only) |
| Info Disclosure | Leaks in logs/errors | Redaction, sanitization |
| DoS | Large logs/files | Size/time limits |
| Elevation of Privilege | Docker/socket abuse | Least privilege, read-only first |

## Where the Controls Live

There is no `internal/security` package. An earlier empty package with that name
was removed: a package with no code suggests a centralised security layer that
does not exist. Each guard is enforced by the code that performs the operation:

| Control | Implementation |
|---|---|
| Path traversal and symlink containment | `internal/fsutil.SafeJoin` resolves symlinks and rejects paths outside the base |
| Atomic file replacement | `internal/fsutil.AtomicWrite` writes a sibling temporary file and renames it into place |
| Directory and file permissions | `internal/config.EnsureDirs` (0750, state dir 0700), `internal/audit` (0700 dir, 0600 log), callers of `AtomicWrite` |
| Privilege-aware paths | `internal/config.Default` selects `/opt/orvyn`, `/etc/orvyn` and `/var/lib/orvyn` when the effective UID is 0, otherwise user-local paths |
| No shell concatenation | `internal/executil` runs allowlisted executables with arguments passed separately and applies a timeout |
| Secret redaction (best-effort) | `internal/redaction`; its coverage and limits are documented below |
| Archive validation | `internal/backup` rejects traversal, symlinks, oversized and unknown entries, and unpacks into a temporary directory before touching destinations |
| Confirmation before destructive work | `internal/cli` prompts in `apply`, `restore` and `uninstall`; `--yes`/`--force` opt out explicitly |
| Audit trail | `internal/audit` records metadata-only events (actor, operation, target, approval, result) |

These controls do **not** include an ownership check: Orvyn does not verify file
ownership and does not refuse to run as root. It relies on the permissions of
the user running it, the confirmation prompts above, and the operating system's
access control.

## Residual Risks
- Redaction is pattern-based (not 100%)
- Local audit can be tampered if host compromised
- Docker socket is highly privileged
