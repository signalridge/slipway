# Assurance

## Scope Summary

This change resolves GitHub issues #195 and #196 for the `slipway status`
surface:

- Active governed changes that are ready for finalization now expose a
  machine-readable `done_ready: true` projection while preserving the persisted
  lifecycle status as `active`.
- Explicit status lookup for a finalized archived change now returns a
  read-only archived status view instead of reporting a missing active bundle as
  `change_state_load_failed`.

The implementation is scoped to the status command/view/rendering path and
focused regression tests listed in `tasks.md`.

## Verification Verdict

Current execution evidence for task `t-01` is passing with fresh task evidence.
Regression coverage was added before implementation and then made green:

- RED: `go test ./cmd -run 'TestBuildGovernedStatusViewExposesDoneReadyReadiness|TestCLIEndToEndSuccessfulDoneArchive' -count=1`
  failed before the implementation because `done_ready` was absent and archived
  lookup returned a missing active change error.
- GREEN: the same focused command passed after implementation.
- GREEN: `go test ./cmd -count=1` passed.
- GREEN: `go test -count=1 ./...` passed.
- GREEN: `git diff --check` passed.
- GREEN: `go test -count=1 -coverpkg=./cmd,./internal/state
  -coverprofile=/tmp/slipway-195-196-cross-cover.out ./cmd ./internal/state`
  passed, with changed behavior covered by focused status and archive tests.

The latest active validation reached `S4_VERIFY` with fresh evidence and a
passing scope contract. Spec-compliance review, code-quality review, and
goal-verification are all recorded as passing for run summary version `1`.
Final ship-readiness still depends on final-closeout recording the required
standard-preset assurance attestation before `slipway done` archives the change.

## Evidence Index

- Task evidence: `verification/execution-summary.yaml` records `t-01` as
  passing with changed files matching planned target files.
- Scope contract: `go run . status --json` and `go run . validate --json`
  report `scope_contract.status=pass`.
- Review evidence:
  - `verification/spec-compliance-review.yaml` records `layer:R0=pass`,
    `scope_contract:pass`, and `negative_path:pass`.
  - `verification/code-quality-review.yaml` records `layer:IR1=pass`.
- Goal evidence: `verification/goal-verification.yaml` records fresh command
  proof, scope-contract proof, and change-surface coverage proof.
- Implementation: `cmd/status.go`, `cmd/status_view_build.go`, and
  `cmd/status_render.go`.
- Regression tests: `cmd/status_view_build_test.go` and `cmd/cli_e2e_test.go`.

## Requirement Coverage

- REQ-001 is covered by the done-ready projection in status view construction,
  JSON/text rendering, the `run_slipway_done_to_finalize` reason, and
  `TestBuildGovernedStatusViewExposesDoneReadyReadiness`.
- REQ-002 is covered by archived fallback status lookup, archived status view
  rendering, and the archived lookup assertions in
  `TestCLIEndToEndSuccessfulDoneArchive`.

## Residual Risks and Exceptions

No residual implementation exceptions are accepted at this point. The primary
risk is accidental drift between text and JSON status surfaces; both are updated
from the same `statusView` projection fields and covered by focused tests.

The archived fallback intentionally preserves active state priority: active
`state.LoadChange` succeeds first when an active change exists, and archived
lookup is used only after active loading fails.

## Rollback Readiness

Rollback is a normal git revert of the status command/view/rendering changes and
their focused regression tests. No dependency, schema migration, external API,
or irreversible data operation is part of this change.

## Archive Decision

Ready for final closeout. Active `validate --json` freshness and scope evidence
has been captured at `S4_VERIFY`; required review evidence and goal-verification
evidence are passing. After final-closeout records
`closeout:assurance_complete=pass` and the ship gate reports done-ready,
`slipway done` should archive the governed bundle under
`artifacts/changes/archived/resolve-github-issues-195-and-196-make-status-expose-done-re/`.
