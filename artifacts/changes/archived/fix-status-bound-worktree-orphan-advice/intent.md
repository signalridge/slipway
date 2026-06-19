# Intent

## Summary
Fix `slipway status --json` so an invocation from inside an active
change's bound worktree does not report that same slug as an orphaned
bundle or recommend deleting it.

## Complexity Assessment
simple

Rationale: the issue is confined to status/change resolution and recovery
advice, but needs careful regression coverage because misleading delete
guidance can block governed closeout.

## In Scope
- `slipway status --json` behavior when cwd is inside a worktree bound to an
  active governed change.
- Change-resolution or orphan-detection logic that causes an active bound
  change to be reported as `orphaned_change_bundle` when status is unscoped.
- Recovery advice for orphan reports, specifically avoiding a primary
  `slipway delete --change <active-slug>` recommendation for the active
  bound worktree.
- Focused regression tests for the issue #266 scenario.

## Out of Scope
- Lattice repository cleanup or mutation.
- Destructive worktree or bundle deletion commands.
- Broad redesign of Slipway lifecycle binding, validation, or closeout gates.
- Unrelated changes to issue #183 notes-file root resolution.

## Constraints
- Preserve existing explicit `--change <slug>` behavior.
- Preserve legitimate orphaned-bundle detection for bundles that are not the
  active bound worktree change.
- Keep the fix narrow enough to ship as an issue-driven hotfix.

## Acceptance Signals
- A regression test demonstrates that unscoped status from an active bound
  worktree resolves to that active change instead of reporting it as an
  orphaned bundle.
- Existing status, validate, and lifecycle tests continue to pass.
- Local verification includes the focused package tests and an appropriate
  repo-wide Go test run.

## Open Questions
- [x] Should unscoped status inside a bound worktree prefer the active bound
  change over broader orphan scanning? Confirmed by user on 2026-06-19.

## Approved Summary
Confirmed by user on 2026-06-19: fix issue #266 by making unscoped
`slipway status --json` safe and useful inside an active change's bound
worktree. The change should report the active bound change, or at minimum
avoid presenting deletion of that active slug as primary recovery. Scope is
limited to status/change-resolution/recovery advice and regression tests;
Lattice cleanup, destructive worktree cleanup, broad lifecycle redesign, and
unrelated issue #183 behavior are out of scope.
