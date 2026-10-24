# Contributing to Orvyn

## Workflow
1. Fork the repository
2. Create a short-lived branch from the default branch
3. Make focused, reviewable changes
4. Add/update tests
5. Run checks locally
6. Open a Pull Request

## Branches
- `main` is the protected integration branch (project policy)
- Direct pushes to `main` are prohibited by project policy
- Use `feat/`, `fix/`, `docs/`, `chore/`, `sec/` prefixes when practical

## Commits
We recommend [Conventional Commits](https://www.conventionalcommits.org/):
- `feat: ...`
- `fix: ...`
- `docs: ...`
- `chore: ...`
- `sec: ...`
- `test: ...`

## Pull Requests
PRs must explain:
- What changed and why
- Tests added/updated
- Risks and mitigations
- Compatibility impact

## Security
- Never commit secrets, customer logs, private keys, or production data
- Security-sensitive changes require explicit security review
- Follow [AGENTS.md](AGENTS.md) and [SECURITY.md](SECURITY.md)

## Local Checks
```bash
gofmt -l .
go vet ./...
go test ./...
```
