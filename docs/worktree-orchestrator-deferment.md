# Worktree And Orchestrator Promotion Decision

Date: 2026-04-10
Status: Accepted

## Decision

Phase 6 from `docs/plans/2026-04-09-functional-optimization-plan.md` is officially
deferred.

Slipway does not currently promote worktrees, lanes, or orchestrator-managed
parallel execution to first-class product surfaces. The supported product
surface stops at the stabilized command model, wave-backed execution state,
recovery diagnostics, and the internal governance agent model from Workstreams
1-5.

This closes the Phase 6 exit criterion by making the boundary explicit:
parallel execution is not a supported product feature on the current release
line.

## Supported Today

- Canonical scope-root ownership across the main checkout and sibling worktrees
- Worktree-bound bundle and runtime path resolution
- Worktree preflight checks for discovery-heavy governed work
- Repair of scope metadata and related worktree integrity issues
- Governance-mapped `slipway-orchestrator` hints during `S2_EXECUTE`
- Manual-only helper agents such as `slipway-analyst` and `slipway-executor`

## Not A Product Contract

- No first-class `worktree`, `lane`, or parallel execution commands
- No automatic multi-worktree scheduling contract
- No Slipway-owned concurrent executor orchestration across multiple worktrees
- No user-facing guarantee that multiple governed execution sessions can run in
  parallel safely under one change

## Rationale

The current codebase already contains internal worktree authority, path
resolution, and orchestrator-oriented agent templates. That is enough to
support governed execution in repositories that use worktrees, but it is not
yet the same as a productized parallel runtime.

Promoting those internals prematurely would create a split state: users would
see worktree concepts as product promises before Slipway has explicit
commands, recovery semantics, and verification coverage for parallel
execution.

Deferring Phase 6 keeps the public boundary honest while preserving the
internal foundations for a later promotion.

## Re-open Conditions

Revisit Phase 6 only when all of the following are true:

1. Workstreams 1-5 remain stable in day-to-day use.
2. A concrete user-facing worktree command model exists.
3. Recovery semantics for concurrent execution are explicit and tested.
4. Parallel execution can be either fully supported or rejected fail-closed.
