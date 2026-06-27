# Research

## Alternatives Considered

### Architecture
- Affected modules: `cmd/state_read_context.go`, `cmd/status.go`,
  `cmd/next.go`, `cmd/validate.go`, `cmd/common.go`,
  `internal/state/store.go`, `internal/state/verification.go`, and
  `internal/state/lifecycle_event.go`.
- Dependency chains: CLI commands call `stateReadContext`, which delegates to
  `internal/state` for authoritative reads; governance readiness still calls
  `internal/engine/progression`, which reads verification data through
  `internal/state`.
- Blast radius: limited to read assembly and state read helpers. Mutation
  paths, lifecycle append semantics, and durable authority stay unchanged.
- Constraints: `internal/state` must remain below `internal/engine`; the read
  context must not become a persistent cache; missing explicit slug and archived
  semantics must continue to fail closed.

### Patterns
- Existing conventions: current code already uses `stateReadContext` for
  `loadChange`, `reloadChange`, `resolvedPaths`, `verificationRecords`, and
  `loadExecution`.
- Reusable abstractions: `state.LoadChangeFast` already prioritizes
  invocation-local/bound authority before falling back to full workspace scans;
  `state.ListVerificationsForChange` avoids slug-based rediscovery after a
  change is resolved; `state.ReadLifecycleEventTailWithPredecessorTransition`
  provides status-display tail reads.
- Convention deviations: some readiness code still calls `progression` directly
  and therefore cannot reuse command-local verification inventory without an
  additional option or adapter. That should be handled as a focused extension,
  not by making `internal/state` depend on engine types.

### Risks
- Technical risks:
  - medium: caching route or verification data can mask malformed or stale
    authority if reused after a mutation inside the same command.
  - medium: explicit `--change` fast paths can accidentally skip hidden sibling
    or archived checks that currently fail closed.
  - low: status tail reads can under-report malformed old log lines if used in
    health/repair surfaces. The current tail helper should remain display-only.
- Guardrail domains: no sensitive auth, financial, credentials, or schema
  migration domain is touched.
- Reversibility: changes are localized and reversible by removing read-context
  reuse and falling back to existing direct state reads.

### Test Strategy
- Existing coverage: `cmd/state_read_context_test.go` covers change/path cache
  behavior; `internal/state/lifecycle_event_test.go` covers tail reads and
  malformed retained lines; `cmd/common_test.go` and
  `cmd/resolve_explicit_change_authority_test.go` cover explicit missing and
  archived semantics.
- Infrastructure needs: add small instrumentation seams or test-only counters
  around state-read context operations rather than broad benchmark-only tests.
- Verification approach: targeted tests should prove explicit successful
  `--change` paths avoid global scans, read context reuses loaded change/paths/
  verification data within a command, and status timeline reads use bounded tail
  behavior while preserving malformed retained-line failures.

### Options
- Option 1: Incrementally complete the existing invocation read context.
  - Design: extend the existing `stateReadContext` and nearby state helpers just
    enough to cover route/worktree-list reuse, explicit-slug fast path tests,
    verification reuse, and status timeline tail behavior.
  - Tradeoffs: smallest blast radius and best fit with current code; may leave
    deeper `progression` readiness reuse for a separate focused seam if it would
    require broad engine API churn.
- Option 2: Build a persistent workspace index.
  - Design: write an index of worktrees, bundles, verification inventory, and
    timeline metadata that commands can reuse across invocations.
  - Tradeoffs: could improve repeated command latency, but violates this
    change's constraint against cross-command authority caches and introduces
    staleness/invalidation risk.
- Option 3: Optimize only status display.
  - Design: stop after tail timeline reads and explicit status fast path.
  - Tradeoffs: low risk, but fails `opt.md` because `next` and `validate` would
    continue to duplicate reads and explicit `--change` behavior would remain
    under-proven.
- Selected: Option 1. It completes the required 4.2/4.3/4.4 state-read scope
  without adding compatibility layers or persistent authority.

## Unknowns
- Resolved: whether tail lifecycle reads already exist -> yes,
  `internal/state/lifecycle_event.go` provides bounded tail helpers and tests.
- Resolved: whether explicit `--change` has a fast load primitive -> yes,
  `state.LoadChangeFast` exists and is used by `stateReadContext.loadChange`.
- Remaining: exact implementation gap will be finalized in plan audit after
  measuring which paths still call global scans in successful explicit cases.

## Assumptions
- The current codebase map is relevant to this scope because it names the exact
  state-read hot paths and tests for this change.
- `opt.md` remains the authoritative scope source for 4.2, 4.3, and 4.4.
- Existing P0 lifecycle semantics are load-bearing and must be preserved rather
  than redefined in this performance change.

## Canonical References
- `opt.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `cmd/state_read_context.go`
- `cmd/status.go`
- `cmd/next.go`
- `cmd/validate.go`
- `cmd/common.go`
- `internal/state/store.go`
- `internal/state/verification.go`
- `internal/state/lifecycle_event.go`
