# Assurance

## Scope Summary

Delivered opt.md 3.2 by adding a high-risk public lifecycle surface coverage
gate alongside the existing governance-kernel gate. The change adds a
`public-surface` `covergate` target for `cmd` and `internal/state`, a committed
`coverage-public-surface-baseline.json` with package/file/surface diagnostics,
CI and local recipe enforcement for both targets, workflow contract coverage,
and contributor documentation.

## Verification Verdict

Pass. The focused coverage tests, CI workflow contract test, full Go test suite,
lint, YAML/action workflow checks, and both coverage gate targets have passed.
During S3 review a code-quality blocker found that check mode could backfill
missing public-surface metadata before validation; this was fixed, covered by a
new regression test, and reverified with focused tests, `go test ./...
-count=1`, `golangci-lint run --timeout 5m ./...`, and both coverage gate
checks.

## Evidence Index

- `verification/task-results/t-01-evidence.md`: RED/GREEN tests for
  public-surface target integrity and diagnostics.
- `verification/task-results/t-02-evidence.md`: target-aware baseline
  implementation, public baseline, and post-review blocker fix evidence.
- `verification/task-results/t-03-evidence.md`: CI, justfile, workflow contract,
  and docs evidence.
- `verification/task-results/t-04-evidence.md`: focused, full-suite, lint, and
  workflow/YAML validation evidence.
- `verification/wave-orchestration-notes.md`: S2 execution summary and
  post-review re-verification summary.
- `verification/code-quality-review-notes.md`: S3 review blocker report that led
  to the fail-closed metadata fix.

## Requirement Coverage

- REQ-001: Public surface coverage target exists in `covergate` and is enforced
  by tests and baseline metadata.
- REQ-002: Fail-closed actionable diagnostics are covered by
  `internal/coverage` and `internal/coverage/cmd/covergate` tests, including
  missing public-surface metadata rejection.
- REQ-003: Kernel and public-surface gates are enforced in CI/local recipes and
  workflow contract tests.
- REQ-004: Baseline/docs are reviewable, no compatibility shim or soft-pass path
  is introduced, and missing public-surface metadata is rejected in check mode.

## Residual Risks and Exceptions

No accepted exceptions. Public-surface coverage is package-level tiered
coverage, not per-line changed coverage; this matches the selected approach and
opt.md 3.2's changed-line-or-tiered allowance.

## Rollback Readiness

Rollback is a normal git revert of this change. Reverting removes the
`public-surface` target, the public baseline, CI/local recipe additions, and docs
updates while restoring the previous kernel-only coverage gate. Verification
after rollback: `go test ./internal/coverage... -count=1` and
`go test ./... -count=1`.

## Archive Decision

Archive is ready after selected S3 reviews and ship-verification pass against
the current active change. Active `validate --json` proof must be captured before
`done`; archived bundles will be treated as frozen records rather than
revalidated active gates.
