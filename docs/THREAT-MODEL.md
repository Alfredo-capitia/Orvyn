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
| Confirmation before destructive work | `internal/cli` prompts in `apply`, `restore`, `down`, `rm`, `prune`, `upgrade` and `uninstall`; `--yes`/`--force` opt out explicitly |
| Compose project ownership | `internal/stack` pins the project name and `internal/cli.verifyProjectOwnership` refuses `down`/`rm`/`uninstall` when the project contains containers without `com.orvyn.managed=true` |
| Audit trail | `internal/audit` records metadata-only events (actor, operation, target, approval, result) |

These controls do **not** include a filesystem ownership check: Orvyn does not
verify file ownership and does not refuse to run as root. The Compose project
ownership check above covers containers, not files. File safety relies on the
permissions of the user running Orvyn, the confirmation prompts, and the
operating system's access control.

## Secret Redaction Limits

`internal/redaction.Redact` is a best-effort filter, not a guarantee. It applies
regular expressions to log lines before the log stream writes them. It does not
inspect files, backups, audit events, command output or error messages that
bypass that path.

Covered by shape:

| Pattern | What is masked | Limitation |
|---|---|---|
| AWS access key ID | `AKIA` followed by 16 uppercase alphanumerics | the secret access key is not recognised |
| JWT | three base64url segments whose first two start with `eyJ` | other token formats are not recognised |
| PEM private key | `BEGIN [RSA ]PRIVATE KEY` through the matching `END` block, both in the same redacted input | the log stream redacts one line at a time, so a multi-line key is not masked; EC, DSA and OpenSSH private keys are not matched |
| Key/value assignments | values after `password`, `passwd`, `pwd`, `secret`, `token`, `api_key`/`apikey`, `auth` or `bearer`, separated by `:` or `=` | values containing spaces are only masked up to the first space |
| Connection strings | `user:password@host` userinfo when the hostname is dotted and ends in a TLD of at least two letters | `localhost`, IP addresses and single-label service names are not matched |

Not covered:

- Provider tokens that do not match those shapes (for example GitHub personal
  access tokens, Slack, Stripe, or generic high-entropy strings).
- Secrets without a recognisable key name.
- Multi-line values, JSON/YAML where quoting or escaping breaks the pattern,
  and base64/hex-encoded or encrypted secrets.
- Any output that does not pass through the log stream: `docker compose` output
  printed by `apply`, files on disk, backups and audit events are not filtered
  by this function.

The filter can also over-redact harmless text that happens to contain one of the
key names. Treat logs as potentially sensitive: avoid writing secrets to log
lines in the first place and restrict access to the log files. Report a
redaction gap as a security issue instead of assuming the filter caught it.

## Residual Risks
- Redaction is pattern-based (not 100%)
- Local audit can be tampered if host compromised
- Docker socket is highly privileged
