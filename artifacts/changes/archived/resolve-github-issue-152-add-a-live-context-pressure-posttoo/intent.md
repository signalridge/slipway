# Intent

## Summary
Resolve GitHub issue #152: add a live context-pressure PostToolUse hook that classifies context utilization and suggests checkpoint without blocking.
## Complexity Assessment
simple
The change is bounded to generated hook surfaces and small classifier behavior.
It reuses the existing static context-budget seam and does not alter lifecycle
state transitions or checkpoint semantics.

## In Scope
- Add a live context-utilization classifier for PostToolUse hook input.
- Add a Claude `PostToolUse` hook registration and generated hook script.
- Emit advisory checkpoint guidance at warn/critical thresholds without blocking
  the tool call.
- Cover the classifier and generated hook/settings contract with Go tests.

## Out of Scope
- No generic automatic compaction engine.
- No change to existing S2 `slipway checkpoint` command semantics.
- No mandatory rollout to every AI runtime beyond the minimal generated surface
  needed for issue #152.
- No sensitive-domain bypass or force-close path.

## Constraints
- Hook behavior must be advisory and fail-silent on missing, malformed, or stale
  context metrics.
- Generated surfaces must remain derived from `internal/toolgen` and
  `internal/tmpl` sources rather than patched generated copies only.

## Acceptance Signals
- A regression test proves Claude settings include a `PostToolUse` hook.
- Classifier tests prove healthy/warn/critical threshold behavior.
- Hook behavior tests prove stale/missing metrics do not block and critical
  metrics suggest `slipway checkpoint`.
- `go test` passes for affected packages.
- Slipway governance reaches `done_ready` without running `slipway done`.

## Open Questions
None.

## Approved Summary
Confirmed by user on 2026-06-10T01:24:24Z: implement #152's minimum
verifiable scope by adding a live context-utilization classifier and a Claude
`PostToolUse` hook that injects checkpoint guidance at pressure thresholds. The
  hook must be advisory, fail-silent, and non-blocking. The change excludes a
  generic compaction engine, checkpoint lifecycle changes, and broad runtime
  expansion.
