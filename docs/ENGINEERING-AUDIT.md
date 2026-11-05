# Engineering Audit — Hardening Programme

Date: 2026-11-05
Repository: `Alfredo-capitia/Orvyn`
Baseline: `0d5b28b` (`dev` before this programme)
Integration reference: `0a0a918` (`dev` after PR #58) plus the open phase PRs

This document records what was found, what was fixed, what was deliberately
left open, and which checks were actually executed. It does not claim a control
that has not been verified.

## 1. Executive summary

| Item | Result |
|---|---|
| Total statement coverage | **46.9% → 64.9%** (local integration measurement) |
| Packages with new or expanded tests | `docker`, `executil`, `config`, `validate`, `cli`, `tui`, `fsutil`, `instancelock` |
| Production defects fixed | 6 (two TUI state bugs, one crash, non-durable atomic writes, false security documentation, timeout not killing descendants) |
| Operational control added | Single-writer instance lock for `apply`, `down`, `rm`, `uninstall` |
| External finding | `CVE-2026-109518` in `grafana/alloy:v1.21.3` — **resolved by #66** (distroless image); `image-scan` is green |
| Follow-up (2026-11-05) | `dev-protection` ruleset created; `main-protection` now requires all four security checks |
| CI/CD finding | **Fixed:** `dev` had no protection and `main` required only `build-and-test`; both now require the four security checks |
| Race detector | Not runnable locally (no C toolchain); CI runs `go test -race` and passes |
| `govulncheck` | Not runnable locally (tool absent, network failure); CI job passes |

No existing test was removed, no framework was added, no security check was
disabled, and no Trivy baseline was edited to hide the Alloy CVE.

## 2. Method and constraints

- One branch and one pull request per phase, all targeting `dev`.
- `gofmt -l .`, `go build ./...`, `go vet ./...` and `go test ./...` before
  every commit.
- No architectural rewrite, no new runtime dependency, no new feature.
- Tests use fake hosts: temporary `HOME`, a fake `docker` script in `PATH`, and
  an `httptest` Docker API. No real daemon, container or customer data is used.
- Production code only changed where a real defect was proven by a failing
  test. `internal/validate` also gained test seams (probe functions stored in
  variables); its production probes and report are unchanged.

## 3. Coverage

Baseline measured on `0d5b28b` with `go test ./... -cover`. Combined value
measured on a local, unpushed integration merge of PRs #59–#64 on top of
`0a0a918` with `go test ./... -coverprofile` and
`go tool cover -func`. The upstream total will only be official after those
PRs are merged.

| Package | Before | After | Phase / PR |
|---|---:|---:|---|
| `internal/docker` | 16.4% | 94.0% | 1 / #58 (merged) |
| `internal/executil` | 33.3% | 92.0% ¹ | 2 / #59, 7 / #64 |
| `internal/config` | 42.9% | 85.7% | 2 / #59 |
| `internal/validate` | 0.0% | 87.8% | 2 / #59 |
| `internal/cli` | 14.3% | 37.0% ² | 3 / #60, 6 / #63 |
| `internal/tui` | 35.9% | 70.9% | 4 / #61 |
| `internal/fsutil` | 45.0% | 54.0% | 5 / #62 |
| `internal/instancelock` | — (new) | 81.4% | 6 / #63 |
| `internal/apply` | 93.3% | 93.3% | unchanged |
| `internal/audit` | 80.0% | 80.0% | unchanged |
| `internal/backup` | 68.1% | 68.1% | unchanged |
| `internal/logs` | 59.1% | 59.1% | unchanged |
| `internal/planner` | 50.0% | 50.0% | unchanged |
| `internal/redaction` | 92.3% | 92.3% | unchanged |
| `internal/stack` | 67.1% | 67.1% | unchanged |
| `internal/discovery` | 0.0% | 0.0% | **still uncovered** |
| `internal/resources` | 0.0% | 0.0% | **still uncovered** |
| `internal/state` | 0.0% | 0.0% | **still uncovered** |
| `cmd/orvyn` | 0.0% | 0.0% | **still uncovered** |
| `pkg/types` | no tests | no tests | — |
| **Total** | **46.9%** | **64.9%** | |

¹ PR #59 measured `internal/executil` at 100.0% before PR #64 added the
process-group code. Two new branches (nil process, already-dead group) are not
exercised, so the combined package is 92.0%.

² PR #60 measured `internal/cli` at 35.3% on its own branch; the combined value
including the instance-lock tests is 37.0%.

## 4. Problems found and fixed

### 4.1 Test coverage gaps (fixed)

| ID | Area | Finding | Fix | Evidence |
|---|---|---|---|---|
| AUDIT-1 | `internal/docker` | Client wrappers were untested (16.4%). | Added a fake Docker API covering list, inspect, logs, exec and error paths. | PR #58, merged; 94.0% |
| AUDIT-2 | `executil`, `config`, `validate` | Execution semantics and preflight branches were untested. | Covered success, non-zero exit, timeout, cancellation, argument passing and interactive mode; `EnsureDirs` idempotency and permissions; all preflight branches via test seams. | PR #59; 100% / 85.7% / 87.8% on that branch |
| AUDIT-3 | `internal/cli` | Destructive commands were only tested for the prompt helper. | Ran the real commands against a fake `HOME`, daemon and `docker`, asserting prompts, `--yes`/`--force`, `--volumes`, audit results, ownership guard and failure paths. | PR #60; 35.3% on that branch |
| AUDIT-4 | `internal/tui` | State transitions and async commands were untested. | Drove `Update` with the messages the program emits: filters, details, refresh, health, logs, keys, commands. | PR #61; 70.9% |
| AUDIT-5 | `internal/fsutil` | Durability and failure paths were untested. | Covered exact mode, overwrite, failed rename cleanup, unusable directory and 16 concurrent writers. | PR #62; 54.0% |

### 4.2 Production defects (fixed)

| ID | Defect | Impact | Fix |
|---|---|---|---|
| AUDIT-6 | `applyFilter` and `buildDetails` used value receivers. | Every mutation happened on a copy and was discarded: the container table and details pane never refreshed. | Pointer receivers; tests exercise the real `Update` path. |
| AUDIT-7 | `fetchHealthCmd` dereferenced `inspect.State` without a nil check. | A partial or malformed inspect response crashed the dashboard. | Skips entries whose `State` or health status is missing. |
| AUDIT-8 | `AtomicWrite` used a fixed `<path>.tmp` and did not `fsync`. | A crash could leave the apply journal or a generated config truncated; concurrent writers shared a temporary name. | Unique `os.CreateTemp` sibling, `chmod`, write, `fsync`, rename, directory `fsync`, cleanup on failure. |
| AUDIT-9 | Documentation claimed every path goes through `SafeJoin`. | Security documentation overstated an enforced control. `SafeJoin` has no production caller; restore uses stricter entry-by-entry checks. | README, `SECURITY.md`, contributor checklist, risk register and threat model now describe the actual controls. |
| AUDIT-10 | `RunWithTimeout` killed only the direct child. | A timed-out command could keep blocking until descendants closed the output pipe. Measured: 5.00s for a 100ms timeout with a background `sleep 5`. | Child starts in its own process group; cancellation sends `SIGKILL` to `-pid`; `WaitDelay` bounds the pipe drain. Measured: 0.10s. |
| AUDIT-11 | Nothing serialised mutating commands. | A concurrent `apply` and `down` could interleave backup, compose rewrite and container removal. | `internal/instancelock` takes an exclusive, non-blocking `flock` on `<state>/orvyn.lock` for `apply`, `down`, `rm` and `uninstall`; refused runs are audited as `result=locked`. |

Each fix has a regression test in the corresponding PR. The process-group fix
was verified both ways: the new test fails in 5.00s with the old behaviour and
passes in 0.10s with the fix.

## 5. Problems found and their disposition

| ID | Finding | Why it is open | Recommendation |
|---|---|---|---|
| OPEN-1 | **Resolved by #66.** `image-scan` failed on `dev` and every PR: `CVE-2026-109518` (HIGH) in `grafana/alloy:v1.21.3`. Installed `apt` / `libapt-pkg6.0t64` `2.8.3`, fixed `2.8.4ubuntu0.1`. Trivy reported 2 HIGH, 0 CRITICAL. | The pinned tag was the newest Docker Hub release (2026-10-28); no exception was added. | Done: the stack now pins `v1.21.3-distroless`, which has the same Alloy binary but no apt package. `image-scan` is green on #66; the earlier PRs inherit the fix once `dev` contains it. |
| OPEN-2 | **Resolved.** `dev` was not protected (`GET /branches/dev/protection` returned `404 Branch not protected`). | Applied with explicit approval. | Done: the `dev-protection` ruleset (id `26618766`) requires a PR, `build-and-test`, `govulncheck`, `image-scan`, `codeql`, resolved review threads, and forbids force-push/deletion. Approvals stay at 0 for the single maintainer. |
| OPEN-3 | **Resolved.** The `main-protection` ruleset only required `build-and-test`; `govulncheck`, `image-scan` and `codeql` were advisory on `main` (`required_approving_review_count: 0`). This is why PR #58 could be merged while `image-scan` was red. | Applied with explicit approval. | Done: `main-protection` (id `25801068`) now requires the same four checks. |
| OPEN-4 | `internal/discovery`, `internal/resources`, `internal/state` and `cmd/orvyn` are at 0% coverage. | Out of scope for the phases completed so far. | Cover discovery filters and resource detection next; add a command smoke test for `cmd/orvyn`. |
| OPEN-5 | `internal/state` has no production caller and hardcodes `/var/lib/orvyn`. | Removing or repurposing it is an architectural decision, not a test fix. | Delete it, or make it use `config.Default()` and wire it to the commands that need persisted state. |
| OPEN-6 | `SafeJoin` has no production caller. | The restore path already uses stricter checks (`safeRel`, `withinRoot`, `ensureNoSymlinks`). | Either use `SafeJoin` at a real call site or remove it; do not keep it as documentation-only security surface. |
| OPEN-7 | The instance lock covers `apply`, `down`, `rm` and `uninstall`, but not `prune`, `upgrade`, `restore`, `start`, `stop`, `restart` or TUI actions. | The phase scope was the four stack-mutating commands. | Extend the same lock to every command that mutates containers or files, including TUI actions. |
| OPEN-8 | `executil` process-group code has two unexercised branches (nil process, already-dead group). | Hard to trigger deterministically without racing the scheduler. | Accept as low risk, or inject a `kill` seam if these branches become security-relevant. |
| OPEN-9 | Two reachable advisories are accepted in `.github/govulncheck-exclude.txt`: `GO-2026-4887` and `GO-2026-4883` in `github.com/docker/docker v28.5.2+incompatible`. | Both are marked "Fixed in: N/A" upstream; the newest published module is pinned. | Revisit by 2027-01-31 or when a fixed release ships. |
| OPEN-10 | Local `-race` and `govulncheck` could not run. | `-race` requires cgo and `gcc` is not installed; `govulncheck` is not installed and the vulnerability database was unreachable from this host. | Use CI evidence until the local toolchain is provisioned; do not install tooling without approval. |

## 6. Race detector

Local execution is not possible in this environment:

```
$ go test -race ./...
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1

$ CGO_ENABLED=1 go test -race ./internal/executil
# runtime/cgo
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in $PATH
```

`gcc` was not installed because that is a host change requiring explicit
approval. CI does run the race detector:

```yaml
- name: Test (race + coverage)
  run: go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...
```

`build-and-test` is green on PRs #59–#64, so the race detector passed on every
phase branch in CI. This is CI evidence, not a local result.

## 7. govulncheck

Local execution was not possible:

- the binary is not installed (`timeout: failed to execute process: No such file or directory`);
- earlier attempts to reach `vuln.go.dev` failed on DNS.

The CI `govulncheck` job is green on PRs #59–#64. It treats only findings with a
function in the trace as reachable, and it fails on any reachable advisory that
is not listed in `.github/govulncheck-exclude.txt`. The two accepted advisories
are documented in that file with the reason and a revisit date (2027-01-31).
This programme added no new exclusion.

## 8. CI/CD assessment

Observed workflow (`.github/workflows/ci.yml`):

| Control | State |
|---|---|
| Triggers | `pull_request` and `push` to `main`, `master`, `dev`; weekly cron |
| Permissions | Workflow default `contents: read`; CodeQL adds `security-events: write` and `actions: read` |
| Actions pinning | All third-party actions are pinned by commit SHA with a version comment |
| Formatting | `gofmt -l .` fails the build |
| Static analysis | `go vet ./...` and pinned `staticcheck v0.8.2` |
| Tests | `go test -race -count=1 -covermode=atomic` |
| Coverage output | Printed (`go tool cover -func`) but no threshold is enforced |
| `govulncheck` | Reachability-aware, allowlist with documented exceptions |
| Image scan | Trivy v0.76.1 per image, HIGH/CRITICAL blocking, per-image baselines, derived from `internal/stack/generate.go`, stale baselines fail |
| CodeQL | Go, pinned CodeQL action |
| Concurrency | `ci-<workflow>-<ref>` with `cancel-in-progress` |

Findings:

1. **`image-scan` was red for an external reason.** The scan iterates the images
   alphabetically: `gcr.io/cadvisor/cadvisor:v0.50.0` passed, then
   `grafana/alloy:v1.21.3` failed. Because the job uses `set -e`, the remaining
   images (`loki`, `node-exporter`, `prometheus`) were not re-validated in that
   run. **Resolved by #66**: the distroless tag passes the same scan.
2. **PR #58 was merged with `image-scan` failing** (`build-and-test`,
   `govulncheck` and `codeql` were green). This showed that the repository
   settings did not enforce the security jobs.
3. **`dev` had no branch protection.** **Fixed:** the `dev-protection` ruleset
   now requires the four checks.
4. **`main` required only `build-and-test`.** **Fixed:** `main-protection` now
   requires `govulncheck`, `image-scan` and `codeql` as well.
5. **Coverage is reported but not gated.** The number can regress silently.
6. **No integration test runs the built binary.** `cmd/orvyn` is untested.

## 9. Residual risks

| Risk | Severity | Status |
|---|---|---|
| Alloy base-image CVE reaches production images | High | Resolved by #66 (distroless); future findings block `image-scan` |
| A security job can regress without blocking `dev` or `main` | High | Mitigated: all four checks are required on `dev` and `main` |
| Two mutating commands outside the lock interleave | Medium | Open; OPEN-7 |
| Uncovered discovery/resource/state code hides regressions | Medium | Open; OPEN-4 |
| `internal/state` writes to a hardcoded root-owned path if ever wired | Medium | Open; OPEN-5 |
| Accepted Docker advisories remain reachable | Medium | Documented until 2027-01-31; OPEN-9 |
| Local race/govulncheck gaps reduce pre-push confidence | Medium | Mitigated by CI, not by local runs; OPEN-10 |
| Atomic write is durable but callers do not fsync parent of newly created directories | Low | Accepted; `AtomicWrite` fsyncs the immediate directory |
| Secret redaction remains best-effort | Medium | Pre-existing and documented in `docs/THREAT-MODEL.md` |

## 10. Recommendations

Priority order:

1. **Resolve or date the Alloy CVE (OPEN-1).** Done in #66: switched to the
   distroless variant instead of adding an exception. Keep the tag current.
2. **Protect `dev` and strengthen `main` (OPEN-2/OPEN-3).** Done: both rulesets
   now require `build-and-test`, `govulncheck`, `image-scan` and `codeql`.
   Approvals remain at 0 because the project has a single maintainer.
3. **Extend the instance lock (OPEN-7)** to every mutating command and to TUI
   actions.
4. **Cover the remaining 0% packages (OPEN-4)**, starting with discovery and
   resource detection, then a `cmd/orvyn` smoke test.
5. **Remove or wire `internal/state` (OPEN-5)** so the repository has no unused
   storage layer with a hardcoded privileged path.
6. **Decide on `SafeJoin` (OPEN-6)**: use it at a real call site or remove it.
7. **Add a coverage floor** to CI (for example, fail below the current total) so
   the gain is not lost.
8. **Provision the local security toolchain** (cgo/`gcc`, `govulncheck`) so
   contributors can reproduce CI before pushing.

## 11. Verification performed

Commands run locally on each phase branch before commit:

```
gofmt -l .
go build ./...
go vet ./...
go test ./... -count=1
```

Additional measurements:

```
# Baseline (0d5b28b)
go test ./... -cover              -> total 46.9%

# Integration merge of PRs #59-#64 on 0a0a918 (not pushed)
go test ./... -count=1 -coverprofile=/tmp/opencode/cover.out
go tool cover -func=/tmp/opencode/cover.out | tail -1   -> total 64.9%

# Evidence that the timeout fix is real
# old behaviour: 100ms timeout returned after 5.00s
# new behaviour: 100ms timeout returned after 0.10s
```

Checks not executed locally: `go test -race`, `govulncheck`, `staticcheck`.
Their CI counterparts are green on the phase PRs; `image-scan` is red for the
Alloy CVE.

## 12. Phase index

| Phase | Branch | PR | Scope |
|---|---|---|---|
| 1 | `test/docker-client` | #58 (merged) | Docker client tests |
| 2 | `test/executil-config-validate` | #59 | Execution, config, preflight |
| 3 | `test/cli-destructive` | #60 | Destructive command paths |
| 4 | `test/tui-concurrency` | #61 | TUI state + two fixes |
| 5 | `fix/fsutil-durability` | #62 | Durable atomic writes + docs |
| 6 | `feat/instance-lock` | #63 | Single-writer lock |
| 7 | `fix/executil-process-group` | #64 | Timeout kills the process group |
| 8 | `docs/engineering-audit` | this document | Audit report |
