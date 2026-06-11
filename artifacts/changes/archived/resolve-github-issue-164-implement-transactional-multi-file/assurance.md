# Assurance

## Scope Summary

This change implements issue #164 for transactional governed multi-file
mutations. The selected option was the focused `internal/fsutil` file
transaction helper, applied to the governed transition surfaces identified in
research:

- S1 planning bundle scaffold before `change.yaml` authority persistence.
- S1-to-S2 `wave-plan.yaml` materialization before `change.yaml` authority
  persistence.
- Stale-evidence recovery removals and digest pruning before reopened
  `change.yaml` authority persistence.

The implementation also includes the S3 review fix for scaffold-owned artifact
paths: direct symlink artifact paths are rejected before transaction-backed
atomic writes can replace them.

Out of scope remains unchanged from the approved decision: directory archive,
bundle relocation, crash-recovery journaling, and broader lifecycle redesign.

## Verification Verdict

Pass. The governed execution, spec-compliance review, and code-quality review
all report pass for run version 1. `go run . validate --json` currently reports
state `S4_VERIFY`, freshness `fresh`, and scope contract `pass`; after
goal-verification, the remaining blocker is final-closeout evidence and its
standard-preset assurance attestation.

Fresh verification evidence recorded through S4 verification:

- `go test ./internal/fsutil ./internal/engine/artifact ./internal/engine/progression ./internal/state`: pass
- `go test ./cmd -count=1`: pass
- `go test ./... -count=1`: pass
- `git diff --check`: pass
- `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=sarif -out=artifacts/changes/resolve-github-issue-164-implement-transactional-multi-file/verification/gosec.sarif ./...`: pass; SARIF result count is 0
- `go test ./internal/fsutil ./internal/engine/artifact ./internal/engine/progression ./internal/state -coverprofile=artifacts/changes/resolve-github-issue-164-implement-transactional-multi-file/verification/coverage.out -count=1`: pass; combined profile total is 70.3%
- `go run . validate --json`: fresh, scope contract pass

Post-archive PR lint cleanup:

- GitHub PR #181 initially reported `Lint Go` failure for unused helper
  functions left behind after the transactional recovery path replaced the
  older direct-delete helpers.
- The cleanup removes only those unused helpers; the transaction behavior and
  rollback call sites are unchanged.
- GitHub PR #181 then reported a Windows-only test assertion mismatch because
  POSIX permission bits are not reported the same way on Windows. The test keeps
  the restored-file content assertion on every OS and limits the exact
  permission-bit assertion to non-Windows platforms.
- Fresh local verification after the cleanup: `golangci-lint run --timeout=5m`
  pass and `go test ./... -count=1` pass.

## Evidence Index

- `verification/execution-summary.yaml`: run_summary_version 1, tasks `t-01`
  through `t-03` pass.
- `verification/wave-orchestration.yaml` and
  `verification/wave-orchestration-notes.md`: S2 execution pass with TDD red and
  green evidence for all planned tasks.
- `verification/spec-compliance-review.yaml` and
  `verification/spec-compliance-review-notes.md`: pass; forward and reverse
  trace cover REQ-001 through REQ-005 and all changed implementation/test files.
- `verification/code-quality-review.yaml` and
  `verification/code-quality-review-notes.md`: pass; implementation quality,
  test quality, IR3 guardrail safety, and consistency checks have no open
  blockers.
- `verification/goal-verification.yaml` and
  `verification/goal-verification-notes.md`: pass; acceptance criteria are
  verified with fresh full-suite, SAST, placeholder-scan, coverage, and scope
  evidence, including
  `high_risk_check:irreversible_operations.safety_baseline=pass`.
- `verification/gosec.sarif`: current SAST artifact with 0 results.
- `verification/coverage.out`: current target-package coverage profile.
- `requirements.md`, `decision.md`, and `tasks.md`: approved contract, selected
  approach, and completed task coverage.

## Requirement Coverage

- REQ-001 is covered by `internal/fsutil/transaction.go` and regression tests
  proving created-file cleanup and original-byte restoration after later
  failures.
- REQ-002 is covered by transactional stale-evidence recovery in
  `internal/engine/progression/stale_evidence_recovery.go`, including tests that
  restore evidence files, digest state, and lifecycle authority when reopened
  state save fails.
- REQ-003 is covered by `FileTransactionError`, which preserves the original
  operation error and rollback errors with affected file paths, with tests for
  rollback-failure diagnostics.
- REQ-004 is covered by applying the shared file-set transaction boundary to S1
  bundle scaffold, S1-to-S2 wave-plan materialization, and stale-evidence
  reopen recovery.
- REQ-005 is covered by deterministic injected-failure tests for core helper
  rollback, governed scaffold rollback, wave-plan rollback, and stale-evidence
  recovery rollback.

## Residual Risks and Exceptions

No open readiness blocker is known after the review fixes and verification
commands above.

Accepted residual limits:

- The transaction helper is synchronous and process-local; it does not add a
  durable crash-recovery journal.
- Directory archive and bundle relocation remain outside this issue's scope.
- If rollback itself fails at runtime, the command fails closed and reports the
  affected path for manual inspection before normal Slipway validation and
  gates continue.

## Rollback Readiness

Code rollback is straightforward: revert the `internal/fsutil` transaction
helper, the transaction-op call-site changes, and the regression tests added for
issue #164. After a revert, rerun the targeted package tests and
`go run . validate --json` to refresh the governed bundle state.

Runtime rollback readiness is built into the implementation: failed file-set
operations restore previously applied writes/removes in reverse order; rollback
failures are not hidden and do not allow a governed transition to report
success.

## Archive Decision

The active worktree reached `done_ready` through Slipway's public lifecycle
output, and `go run . done --json` archived the governed bundle as a terminal
record. Post-archive PR lint cleanup is documented above rather than restamping
engine-owned evidence.
