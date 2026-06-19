# Assurance

## Scope Summary

This change fixes GitHub issue #266. Unscoped `slipway status --json` now checks
whether the current git worktree is bound to an active governed change before it
runs global orphan/stale delete-recovery diagnostics. When the current worktree
has a valid active binding, status renders that active change. When the binding
does not resolve to active authority, including a fully removed bundle, status
falls back to the existing `ListChanges` and delete-recovery path.

The implemented scope is limited to:
- `cmd/status.go`
- `cmd/delete_test.go`
- `internal/state/worktree_binding.go`
- `internal/state/worktree_binding_test.go`
- `artifacts/changes/fix-status-bound-worktree-orphan-advice/verification/implementation.md`

## Verification Verdict

Verdict: pass.

Current active validation at `2026-06-19T12:38:12Z` reported
`evidence_freshness=fresh` and `scope_contract.status=pass`. The only remaining
blockers at assurance authoring time were the expected assurance/final-closeout
records that this closeout sequence is now producing.

The final full-suite proof is `go test -p 1 ./...`, with transcript digest
`sha256:e7a2352e35075cd5975ca2d59fda08935eefbd1f34888bc0dbeea5ceddca43b2`.
Focused and package verification also passed for the changed `cmd` and
`internal/state` paths after the S3 repair.

## Evidence Index

- `verification/implementation.md`: implementation summary, root cause, repair
  summary, focused/package/full-suite verification commands.
- `verification/suite-result.yaml`: run summary version 1 and full-suite digest.
- `verification/execution-summary.yaml`: S2 execution summary for tasks `t-01`
  and `t-02`.
- `verification/spec-compliance-review.yaml`: pass with `layer:R0=pass`,
  `scope_contract:pass`, and `negative_path:pass`.
- `verification/code-quality-review.yaml`: pass after repair with
  `layer:IR1=pass`,
  `context_origin:stage=review=quality-review-status-bound-worktree-after-repair`,
  and `context_origin:stage=fix=repair-worker-status-bound-worktree-delete-recovery`.
- `verification/independent-review.yaml`: pass from fresh independent review.
- `verification/goal-verification.yaml`: pass with current implementation command
  reference and `scope_contract:pass`.
- Active `slipway validate --json` proof at `2026-06-19T12:38:12Z`: freshness
  fresh, scope contract pass, only final-closeout/assurance blockers remaining.

## Requirement Coverage

REQ-001 is covered by the current-worktree binding route in `cmd/status.go`,
`state.FindActiveChangeByWorktreeBinding`, and
`TestStatusFromBoundWorktreePrefersActiveAuthorityOverRootOrphanSameSlug`. The
test creates same-slug root orphan residue, runs unscoped status from the bound
worktree, and asserts governed status for the active slug with no
`orphaned_change_bundle` or `slipway delete --change <slug>` output.

REQ-002 is covered by the preserved orphan/stale delete-recovery path and its
negative tests. Coverage includes partial bundle deletion, explicit status for a
partially deleted bundle, full bundle deletion from the root workspace, full
bundle deletion from inside the stale bound worktree, and stale runtime recovery
when another active change also exists.

The selected Option C decision is preserved: the new behavior is a narrow
current-worktree authority preference for unscoped status only. Explicit
`--change` behavior, global orphan scanning, and legitimate delete recovery stay
on their existing paths.

## Residual Risks and Exceptions

No unresolved implementation or review blockers remain. The known test harness
exception is that default `go test ./...` can fail or stall in this repository
because package-level parallelism interacts with cwd/lock-sensitive tests. The
accepted full-suite proof for this change is `go test -p 1 ./...`, which keeps
package-internal parallelism while avoiding cross-package interference. That
command passed after the S3 repair.

There is no guardrail domain for this change, no schema migration, no external
API contract change, and no irreversible runtime operation.

## Rollback Readiness

Rollback is a normal git revert of this change. Reverting restores the previous
unscoped status routing and removes the new regression tests. No data files,
database schemas, external services, or migration state are changed.

If rollback is required, run the focused status/delete recovery tests and
`go test -p 1 ./...` after the revert to confirm the repository returns to the
intended baseline.

## Archive Decision

Archive readiness: ready after final-closeout stamps pass and a fresh active
`validate --json` proof is captured immediately before `slipway done`.

The active validation proof captured at `2026-06-19T12:38:12Z` was taken before
`done` and showed fresh evidence and a passing scope contract. Final-closeout
will rerun the active freshness/readiness check after this assurance artifact is
present. The archive record should be treated as a frozen completed-change
record after `done`; archived bundles are not revalidated through the active
validate gate.
