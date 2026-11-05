# Security Checklist

## Before Implementation
- [ ] Affected components identified
- [ ] Security boundaries mapped
- [ ] Permissions minimized
- [ ] Sensitive data identified
- [ ] Side effects assessed
- [ ] Tests for abuse cases planned

## During Implementation
- [ ] Input validated
- [ ] No shell concatenation
- [ ] Secrets not logged
- [ ] Paths stay under a validated root (`safeRel`/`withinRoot`/`ensureNoSymlinks` or equivalent)
- [ ] Resource limits applied
- [ ] Errors sanitized

## Before Completion
- [ ] gofmt, go vet pass
- [ ] Tests pass
- [ ] Static analysis reviewed
- [ ] Diff inspected for secrets
- [ ] Security checks run
- [ ] Residual risks disclosed
- [ ] Regression tests added
