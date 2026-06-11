# Assurance

## Scope Summary

Delivered GitHub issue #185 by changing S4 digest input construction so
`goal-verification` and `final-closeout` do not self-stale when recording
engine-owned `EvidenceRefs` mutates the current
`artifacts/changes/<slug>/change.yaml`.

Implementation stayed scoped to:

- `internal/engine/progression/evidence_digests.go`
- `internal/engine/progression/evidence_digests_test.go`

The governed bundle and codebase map were refreshed for #185.

## Verification Verdict

Current verification evidence is pass:

- Focused regressions passed after failing coverage gaps were identified:
  S4 goal/final evidence-ref normalization and the default raw-content path.
- Full `internal/engine/progression` package test passed.
- Full repository `go test -count=1 ./...` passed.
- Progression package coverage profile passed with 70.6% statement coverage;
  the changed digest helpers are covered, including
  `goalVerificationContentPathInputHash` at 100.0%.
- `git diff --check` passed.
- Runtime task evidence exists for `t-01`, `t-02`, and `t-03`.

## Evidence Index

- `go test -count=1 ./internal/engine/progression -run 'Test(GoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation|DefaultContentDigestKeepsRawChangeYAMLHash)'`
- `go test -count=1 ./internal/engine/progression`
- `go test -count=1 ./...`
- `go test -count=1 -coverprofile=/tmp/slipway-185-progression.cover ./internal/engine/progression`
- `go tool cover -func=/tmp/slipway-185-progression.cover`
- `git diff --check`
- `go run . validate --json`
- `verification/wave-orchestration.yaml`
- `verification/spec-compliance-review.yaml`
- `verification/code-quality-review.yaml`
- Runtime task evidence:
  - `.git/slipway/runtime/changes/resolve-github-issue-185-prevent-s4-goal-verification-from-s/evidence/tasks/t-01.json`
  - `.git/slipway/runtime/changes/resolve-github-issue-185-prevent-s4-goal-verification-from-s/evidence/tasks/t-02.json`
  - `.git/slipway/runtime/changes/resolve-github-issue-185-prevent-s4-goal-verification-from-s/evidence/tasks/t-03.json`

## Requirement Coverage

- REQ-001: covered by
  `TestGoalAndCloseoutDigestIgnoresEvidenceRefOnlyChangeYAMLMutation`, which
  records `EvidenceRefs` for both `goal-verification` and `final-closeout` and
  expects no `change.yaml` stale blocker.
- REQ-002: covered by the same test mutating `Description` after evidence and
  expecting `required_skill_stale:<skill>:artifacts/changes/<slug>/change.yaml`.
- REQ-003: covered by path scoping in `currentChangeAuthorityInput` and the
  `TestDefaultContentDigestKeepsRawChangeYAMLHash` regression for non-goal raw
  content hashing, plus the existing
  `TestStoredGoalDigestStalesWhenInputContentChanges` behavior for a normal
  target file.

## Residual Risks and Exceptions

- Existing digests stamped before this code change may stale once because the
  current-change `change.yaml` digest format changed from raw bytes to
  structured authority-without-`EvidenceRefs`. Re-recording evidence through the
  supported CLI is the expected recovery.
- No manual Lattice artifact edits or public restamp command are included in
  this change.

## Rollback Readiness

Rollback is source-only: revert the helper and regression test changes, then run
`go test -count=1 ./internal/engine/progression` to confirm the previous
behavior is restored. No data migration or cleanup command is required.

## Archive Decision

Not archived yet. Active validation and review evidence must be refreshed after
S3 review and S4 verification before `slipway done` is appropriate.
