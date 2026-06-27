# Decision

## Alternatives Considered

1. Complete the existing invocation-local read context.
   - Pros: small blast radius, matches current code structure, preserves
     `internal/state` as the read layer, and avoids cross-command authority.
   - Cons: requires focused seams/tests to prove successful explicit paths avoid
     global scans.
2. Add a persistent workspace index.
   - Pros: could improve repeated command latency across invocations.
   - Cons: introduces stale authority risk, cache invalidation complexity, and
     violates the scope constraint against persistent caches.
3. Optimize only `status`.
   - Pros: fastest path to visible timeline and status latency improvements.
   - Cons: fails the requirement that `status`, `next`, and `validate` share
     read-context improvements.

## Selected Approach

Select option 1. Extend the existing `stateReadContext` and direct state helper
usage only where required to complete `opt.md` 4.2, 4.3, and 4.4. Do not retain
compatibility wrappers for replaced behavior, and do not introduce persistent
indexes or cross-command caches.

## Interfaces and Data Flow

- `cmd/state_read_context.go` remains the command-scoped read reuse boundary.
- `cmd/status.go`, `cmd/next.go`, `cmd/validate.go`, and shared command
  resolvers should pass the same read context through one command invocation.
- `internal/state/store.go` continues to own `LoadChangeFast`, bundle
  discovery, and fail-closed state authority.
- `internal/state/verification.go` continues to own verification inventory
  parsing, with resolved-change reads preferred after authority is known.
- `internal/state/lifecycle_event.go` continues to own bounded lifecycle tail
  decoding for display surfaces and full-log decoding for integrity surfaces.

## Rollout and Rollback

Rollout is source-only and covered by tests. Rollback is a normal git revert of
the implementation commit; because no persistent cache or schema migration is
introduced, rollback has no data cleanup step.

Verification command for rollback safety:

```bash
go test ./cmd ./internal/state ./internal/engine/progression -count=1
go test ./... -count=1
```

## Risk

- Risk: over-caching within a command could serve stale data after a mutating
  operation. Mitigation: keep the context command-scoped and invalidate dependent
  caches on explicit reload.
- Risk: fast paths could skip archived or hidden sibling checks. Mitigation:
  preserve existing explicit resolver fallback behavior and add regression
  tests for missing/archived semantics.
- Risk: tail timeline reads could hide malformed older entries. Mitigation:
  use tail reads only for bounded display and keep full-log validation on
  integrity/health surfaces.
