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

## Residual Risks
- Redaction is pattern-based (not 100%)
- Local audit can be tampered if host compromised
- Docker socket is highly privileged
