# Privacy Policy

## Principles
- Data minimization
- Purpose limitation
- Local processing only by default
- No external telemetry

## Data Handling
- Only collect what's needed for active operation
- Treat logs as sensitive
- Redact before display/persistence
- Don't read .env, secrets, source code by default
- No automatic uploads

## Storage
- State in /var/lib/orvyn/ with restrictive permissions
- Backups in /opt/orvyn/backups/ or configured location
- Audit in JSONL, metadata only
