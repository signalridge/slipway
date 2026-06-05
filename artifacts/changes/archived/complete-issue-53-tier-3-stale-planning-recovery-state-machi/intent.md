# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI
- Languages: Go
- Test Command: go test -count=1 ./...
- Build Command: go build ./...
- Conventions: 

## Summary
INT-001: Complete issue #53 Tier 3 stale-planning recovery state-machine fixes.

Complete issue #53 Tier 3 stale-planning recovery state-machine fixes for
B1/B2: non-destructive S3/S4 recovery when planning evidence becomes stale.
## Complexity Assessment
critical
Rationale: changes lifecycle state-machine behavior and machine-readable CLI
surfaces for governed recovery paths in an external API contract domain.

## Guardrail Domains
external_api_contracts

## In Scope
- Design and implement a non-destructive S3/S4 recovery path for benign
  planning drift caused by edits to `tasks.md` or planning evidence.
- Make the recovery route actionable from CLI/AI surfaces instead of emitting a
  dead-end instruction to rerun plan-audit from states that cannot call it.
- Align `EvaluateGPivot` and `slipway pivot --rescope` preconditions so allowed
  rescope states are consistent across gate evaluation and command enforcement.
- Preserve useful execution evidence during rescope/recovery while invalidating
  only downstream evidence that depends on stale planning artifacts.
- Add focused regression tests for S3/S4 stale-planning recovery routing,
  recovery ordering, fail-closed stale evidence, and pivot precondition
  consistency.

## Out of Scope
- Tier 1 diagnostics/workflow fixes already merged in PR #60.
- Tier 2 task-ledger and closeout reuse fixes already merged in PR #63.
- Reopening the `execution-summary.broken.*.yaml` backup concern unless a new
  Tier 3-specific repro appears.
- Archiving/closeout via `slipway done`; this change must stop before closeout.

## Constraints
- Keep recovery non-destructive: do not require `cancel` or manual runtime
  surgery for benign S3/S4 planning drift.
- Keep stale planning and scope-contract drift fail-closed until refreshed
  plan-audit, wave-plan, and execution-summary evidence are complete.
- Respect existing Slipway lifecycle gates and preserve unrelated local dirt.
- Treat JSON/CLI behavior changes as external API contract changes requiring
  domain-aware review evidence.

## Acceptance Signals
- Editing `tasks.md` or planning evidence in S3/S4 exposes an actionable
  recovery route rather than a dead-end plan-audit instruction.
- Recovery refreshes plan-audit, wave-plan, and execution-summary evidence in
  the correct dependency order.
- Scope contract drift and stale planning evidence remain blockers until the
  refreshed evidence chain is complete.
- `EvaluateGPivot` and `slipway pivot --rescope` agree on allowed states.
- Fresh proof includes focused regressions plus `go test -count=1 ./...`,
  `go build ./...`, `git diff --check`, and `slipway validate --json`.

## Open Questions
None.

## Resolved Clarifications
Code research selected a non-destructive S3/S4 `slipway run` recovery
transition back to `S1_PLAN/audit`, with pivot rescope kept S2-only.

## Deferred Ideas
- Broader lifecycle ergonomics beyond B1/B2.
- Additional recovery surfaces for non-benign or destructive scope changes.

## Approved Summary
Confirmed from the user objective and GitHub issue #53 comment
`4604547893` on 2026-06-03T05:20:34Z: implement Tier 3 only. The change will
add a non-destructive S3/S4 stale-planning recovery path for B1/B2, align
`EvaluateGPivot` and CLI `pivot --rescope` state preconditions, and preserve
useful execution evidence while invalidating only evidence downstream of stale
planning artifacts. Tier 1, Tier 2, backup handling, and `slipway done`
closeout are out of scope. Acceptance is proven by focused regressions plus
full build/test/diff/governance validation, stopping at close-ready before
final closeout.
