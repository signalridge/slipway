# Intent

## Summary
repair route freshness capability gaps

## Complexity Assessment
simple

The change is simple because the current code already routes `run` through the
shared next-view gate and stops on blockers. The remaining work is a focused
regression hardening pass over public lifecycle JSON surfaces, not a new
capability framework, performance rewrite, or release/security redesign.

## In Scope
- Confirm the two attached analyses against current `main` before treating any
  item as actionable.
- Add regression coverage for default compact `run --json` output so host
  capability requirements, freshness fields, blockers, recovery, and
  confirmation remain visible outside `--diagnostics`.
- Add or tighten regression coverage proving blocker-driven review-alignment
  handoff decisions stay consistent across `status --json`, `validate --json`,
  `next --json`, `next --json --diagnostics`, and `run --json`.
- Apply the smallest code change needed if those regressions expose a real
  current behavior gap.

## Out of Scope
- Do not rework StateReadContext or WorkspaceIndex performance in this change.
- Do not add CI scheduling or perf-baseline automation.
- Do not move state-layer semantic ownership into engine/wave/freshness
  packages.
- Do not treat already-closed current-main items as bugs, including the old
  claim that `run` does not use the shared next-view capability gate.
- Do not change GitHub release protection, rulesets, or supply-chain workflow
  policy.

## Constraints
- Preserve unrelated root worktree dirt (`.gemini/`, `coverage.out`, `opt.md`,
  and `opt-gap-analysis.md`).
- Keep the fix inside the provisioned governed worktree for
  `repair-route-freshness-capability-gaps-2`.
- Prefer existing command/view helpers over duplicating command-specific
  lifecycle logic.
- Use current worktree behavior and tests as authority; do not rely on stale
  opt-gap notes when they contradict current code.

## Acceptance Signals
- Focused `go test` coverage demonstrates default `run --json` carries the same
  host capability, blocker, freshness, recovery, and confirmation contract as the
  diagnostic path when review host capability is unavailable.
- Focused `go test` coverage demonstrates blocker-driven review alignment
  produces consistent current action and handoff targets across public JSON
  surfaces.
- `go test -count=1 ./cmd` passes.
- `go run . validate --json` and `go run . next --json --diagnostics` in the
  governed worktree report the change ready for the next lifecycle step after
  required evidence is recorded.

## Open Questions
None.

## Approved Summary
The user requested deep confirmation and complete repair of the remaining
reported route/freshness/capability issues from two attached analyses. Current
fact checking narrows the repair to regression hardening for the remaining
public lifecycle surface risks: default compact `run --json` capability
handoff output and cross-surface blocker-driven review-alignment consistency.
Already-fixed or unproven items, performance work, CI automation, release
hardening, and broader state/engine architecture changes are out of scope for
this change.
