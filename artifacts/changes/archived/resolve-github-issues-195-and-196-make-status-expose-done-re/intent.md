# Intent

## Summary
Resolve GitHub issues #195 and #196: make status expose done-ready readiness and archived-change status without false missing active bundle repair guidance.

## Complexity Assessment
simple
Rationale: both issues target the same CLI status boundary. The likely change is
localized to status projection and archived-bundle lookup/error classification,
with focused command-output tests.

## In Scope
- `status --json` top-level output after a `done.ready` lifecycle event, so
  operators can see that all gates passed and `slipway done` is the remaining
  action without inspecting the timeline.
- `status --json --change <slug>` behavior when the active bundle is missing
  because the change was finalized into `artifacts/changes/archived/<slug>`.
- Regression tests for the done-ready and archived-change status surfaces.

## Out of Scope
- Changing `slipway done` archival mechanics.
- Reopening or repairing historical Lattice bundles.
- Broad lifecycle-state model redesign outside the status-facing projection.

## Constraints
- The current worktree's Slipway behavior is authority.
- Archived changes must not be presented as active corruption when an archived
  bundle exists.
- Done-ready must remain a readiness/finalization signal, not a false completed
  state before `slipway done` runs.

## Acceptance Signals
- Focused tests cover status output for `done.ready` after S4 verification.
- Focused tests cover `status --change <slug>` when only the archived bundle
  exists.
- `go test ./...` passes.
- `go run . validate --json` reports the governed change ready to advance or
  identifies only expected lifecycle evidence steps before final closeout.

## Open Questions
None.

## Approved Summary
Approved by the user's request to resolve open issues #195 and #196. The change
will make `status --json` expose done-ready readiness after all gates pass, and
will make archived changes report as archived/done or explicitly out of active
status scope instead of `change_state_load_failed` for a missing active bundle.
Scope excludes changing archival mechanics, historical bundle repair, and broad
lifecycle model redesign. Primary acceptance is focused status regression tests
plus the repository test suite.
