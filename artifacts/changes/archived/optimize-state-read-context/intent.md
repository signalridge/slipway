# Intent

## Summary
Optimize state-reading performance for public lifecycle commands by introducing
an invocation-scoped read context, explicit `--change` fast paths, and
tail-oriented status timeline reads.

## Complexity Assessment
complex

Rationale: this touches shared command routing, lifecycle authority reads,
verification inventory reads, status timeline rendering, and performance
measurement fixtures. The change must preserve fail-closed behavior for bound
worktrees, archived changes, missing explicit slugs, multi-active workspaces,
and malformed lifecycle logs.

## Guardrail Domains
None detected.

## In Scope
- Establish a build-binary state-read performance baseline for root `status
  --json`, bound `status --json`, bound `next --json --diagnostics`, bound
  `validate --json`, and explicit `--change <slug>` scenarios.
- Add a single-command read context or equivalent shared authority object for
  `status`, `next`, and `validate` so route resolution, worktree inventory,
  `change.yaml` loading, verification inventory, and lifecycle timeline reads
  are not repeated inside one invocation.
- Add an explicit `--change <slug>` fast path that avoids full global bundle
  scans on the common success path while preserving missing/archived/bound
  fail-closed semantics.
- Add tail-oriented status timeline reading for the displayed event window, with
  malformed lifecycle logs still reported through existing diagnostics.
- Add targeted tests and/or benchmarks proving the performance and correctness
  contracts.

## Out of Scope
- No cross-command persistent cache, durable index, daemon, or authority that can
  outlive one CLI invocation.
- No compatibility layer for retired route, freshness, or timeline APIs.
- No rewrite of mutating lifecycle append crash-safety, fsync, or compaction
  semantics.
- No release workflow, GitHub ruleset, or branch protection changes.
- No unrelated CLI polish or broad refactors outside the state-read hot path.

## Constraints
- Current worktree Slipway behavior is lifecycle authority.
- Cached facts must fail closed to real reads when unavailable or invalid; they
  must never hide stale authority.
- Performance evidence must use a built binary, not `go run`.
- User explicitly instructed that future implementations must not retain
  compatibility layers.

## Acceptance Signals
- A before/after performance artifact records real/user/sys timings, worktree
  count, `change.yaml` count, and verification record count for the required
  command matrix.
- `status`, `next`, and `validate` share the invocation-scoped read context on
  their hot paths.
- Explicit `--change <slug>` no longer scans all bundles on normal success, but
  missing explicit slugs still return `change_not_found`/exit 3.
- `status` timeline rendering uses a tail-oriented read for the displayed event
  count, and malformed logs still fail closed.
- Targeted tests cover no stale cross-invocation reuse, explicit missing slug,
  archived/bound semantics, and malformed timeline logs.
- Final verification includes targeted tests and `go test ./... -count=1`.

## Open Questions
- [x] Identify the current duplicated read paths and the smallest shared
  invocation context boundary that covers `status`, `next`, and `validate`
  without broad API churn.
- [x] Decide how to build a repeatable synthetic fixture with at least 25
  worktrees, 300 `change.yaml` files, and 100 verification records without
  committing bulky generated data.
- [x] Determine the safest tail-read implementation for JSONL lifecycle logs
  that still detects malformed retained lines.

## Deferred Ideas
- Persistent worktree/bundle index.
- Lifecycle log compaction or append crash-safety redesign.
- Full benchmark suite integration beyond a manually repeatable state-read
  benchmark artifact.

## Approved Summary
Confirmed by user authorization on 2026-06-27.

Implement opt.md 4.1-4.4 as one governed performance change: measure the current
state-read baseline with a built binary, introduce an invocation-scoped read
context shared by `status`/`next`/`validate`, add explicit `--change` fast paths,
and make status timeline rendering tail-oriented. Preserve existing
fail-closed behavior and do not add compatibility layers or persistent
cross-command authority.
