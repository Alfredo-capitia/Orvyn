# Incident Response

## Goals
- Contain, assess, remediate
- Preserve evidence safely
- Avoid leaking secrets

## Steps
1. **Detect** - Identify issue
2. **Contain** - Stop affected operations
3. **Assess** - Review logs/audit (sanitized)
4. **Remediate** - Apply fix with tests
5. **Document** - Record findings and fixes
6. **Prevent** - Add regression test

## Guidelines
- Never share raw secrets
- Use sanitized diagnostics
- Review dependencies if supply-chain suspected
