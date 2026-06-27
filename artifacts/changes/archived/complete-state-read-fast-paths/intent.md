# Intent

## Summary
Complete the remaining required state-read performance work from `opt.md` by
implementing a single-invocation read context, explicit `--change` fast paths,
and tail-oriented status timeline reads.

## Complexity Assessment
complex

Rationale: this touches public lifecycle read paths used by `status`, `next`,
and `validate`, and must preserve bound worktree, archived change, missing slug,
multi-active, and no-active fail-closed semantics while reducing redundant reads.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Implement a command-scoped state/read context for `status`, `next`, and
  `validate` that reuses facts discovered during the current invocation only.
- Reuse git root/common-dir/workspace discovery, worktree list parsing,
  invocation route, loaded `change.yaml`, resolved change paths, verification
  records, lifecycle event path, and status timeline reads within one command.
- Add an explicit `--change <slug>` fast path so successful explicit reads avoid
  scanning all change bundles unless fallback or diagnostics require it.
- Add a tail-oriented lifecycle timeline read for `status` so rendering the last
  N events does not require decoding the full lifecycle log.
- Add targeted tests and performance-sensitive regression coverage for the above
  without changing lifecycle authority semantics.

## Out of Scope
- Persistent cross-command caches or durable indexes.
- Compatibility layers for retired read paths or legacy route contracts.
- Release/supply-chain hardening from `opt.md` section 2.
- P0 lifecycle route/freshness/action/capability semantic repairs from
  `opt.md` section 1, except where preserving current semantics is required.
- Lifecycle append crash-safety, fsync, compaction, or JSONL rewrite semantics.

## Constraints
- Current worktree Slipway behavior is the authority; do not substitute
  remembered workflows or source-derived guesses for CLI output.
- No compatibility layer should be retained for removed or replaced
  implementations.
- Cache lifetime must be a single CLI invocation and must fail closed to fresh
  reads rather than serving stale lifecycle authority.
- Preserve unrelated dirty and ignored files.

## Acceptance Signals
- `status`, `next`, and `validate` share a single invocation route/read context
  in the successful bound-worktree paths.
- `status/next/validate --change <slug>` no longer scan all 300+ change bundles
  for the ordinary successful explicit-slug path.
- Missing explicit slug still fails closed with `change_not_found` semantics.
- Archived, bound elsewhere, no-active, and multi-active behavior do not regress.
- `status` timeline tail rendering reads only the required tail for normal
  bounded display while malformed logs still fail closed.
- Targeted tests and `go test ./... -count=1` pass.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
None

## Deferred Ideas
- Persistent workspace index or daemon-backed cache.
- Lifecycle log compaction or append durability redesign.
- Broader public-surface coverage gate changes beyond tests required for this
  state-read optimization.

## Approved Summary
Approved in-session by the user's standing confirmation on 2026-06-27:
implement the remaining `opt.md` state-read performance items 4.2, 4.3, and
4.4 as one coherent change. The change will add command-scoped read-context
reuse, explicit `--change` fast paths, and status timeline tail reads while
preserving fail-closed lifecycle semantics. It will not add compatibility
layers, persistent caches, release hardening, or unrelated P0 lifecycle repairs.
