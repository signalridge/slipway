# Intent

## Summary
enhance execution auto mode
## Complexity Assessment
complex
The change touches public CLI lifecycle behavior, run-loop semantics, governed
handoff output, tests, and generated/documented command surfaces. It is
non-sensitive but correctness-critical because auto mode must never weaken
governance gates.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Define and implement `execution.auto` as auto-to-next-real-gate behavior:
  keep advancing through pass-through lifecycle states and pure-pacing
  continuations when prior authorization is sufficient.
- Preserve hard stops for evidence gates, stale or unknown freshness,
  security-review, sensitive/guardrail confirmations, intake Approved Summary,
  decision/human_action checkpoints, and final `slipway done` archival.
- Update CLI behavior/tests/docs/templates so `slipway run --auto` and
  config-level `execution.auto` have a consistent user-visible contract.
- Keep `slipway next` read-only while making its JSON/handoff output accurately
  describe which boundaries are auto-continuable.

## Out of Scope
- Building a general host/subagent executor inside the Slipway CLI.
- Auto-generating governance evidence, review findings, task results, or
  context-origin attestations without running the owning skill.
- Auto-finalizing or archiving a done-ready change from ordinary `run --auto`.
- Changing sensitive-domain policy, security-review selection, or `slipway done`
  ship-gate verification requirements.

## Constraints
- The current worktree's lifecycle output is authority.
- Existing fail-closed governance semantics must remain intact.
- Any new auto continuation must be machine visible in JSON/handoff output and
  covered by regression tests.
- Keep the implementation scoped to the run/next/stage auto contract and local
  command documentation.

## Acceptance Signals
- Focused tests prove `run --auto` advances through pass-through states to the
  next real gate instead of stopping at routine command boundaries.
- Tests prove hard-stop boundaries are unchanged for intake confirmation,
  security-review, guardrail/sensitive domains, stale/unknown evidence,
  missing required evidence, and done finalization.
- `next --json` remains side-effect-free and reports auto continuation/hard-stop
  status honestly.
- README/reference/generated command surfaces describe the same auto contract as
  the implementation.
- Relevant Go test packages pass after implementation.

## Open Questions
None

## Deferred Ideas
- A future explicit host-level autopilot could consume `next --json`, dispatch
  subagents, and record evidence. That is intentionally separate from the
  Slipway engine/CLI lifecycle authority.
- A dedicated explicit finalize command or flag can be considered separately,
  but ordinary `run --auto` must not archive changes.

## Approved Summary
User confirmed implementation of the investigated direction on 2026-07-02:
enhance `execution.auto` into bounded auto-to-next-real-gate behavior, not
full automation. The change should make auto mode useful for pass-through and
pure-pacing lifecycle pauses while preserving all non-delegable governance
stops: evidence, freshness, security/guardrail, intake approval, decision and
human checkpoints, and final done archival.
