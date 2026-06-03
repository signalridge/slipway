# Decision

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -count=1 ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Option A: Use `pivot --rescope` for S3/S4 recovery
Reuses an existing command, but the current pivot implementation clears
runtime execution residue and returns rescope to S0 intake. That is too
destructive for benign stale-planning drift and would violate REQ-003.

### Option B: Add a new plan-audit refresh command
Creates an explicit callable surface for plan refresh, but it adds another
external API and still needs lifecycle state mutation, evidence invalidation,
and `next` guidance. The extra command is unnecessary for the Tier 3 scope.

### Option C: Reopen `S1_PLAN/audit` from `slipway run`
When S3/S4 readiness reports `stale_planning_evidence`, `slipway run` reopens
`S1_PLAN/audit`, clears only stale derived verification artifacts, preserves
runtime task evidence, and lets the existing plan-audit -> wave-plan ->
execution-summary pipeline rebuild the chain.

### Option D: Allow `pivot --rescope` from S3/S4
This would make review/verify recovery feel direct, but it overloads a
destructive execution-scope pivot with a planning-freshness repair. It would
also make it easier to mutate the object under review after review/verify
started, blurring evidence ownership and invalidation semantics.

## Selected Approach
DEC-001: Use Option C. Implement stale-planning recovery as a governed
`slipway run` transition from S3/S4 back to `S1_PLAN/audit`, with read-only
`next` guidance that tells the operator to run the recovery transition.
Keep pivot separate: align `EvaluateGPivot` to the CLI boundary where rescope
is S2-only because pivot remains destructive, and use `rescope_state_invalid`
as the shared rescope state failure code.

## Interfaces and Data Flow
- `state.LoadRelevantExecutionSummaryContext` continues to classify stale
  planning drift as `stale_planning_evidence`.
- `progression.AdvanceGoverned` handles recoverable S3/S4
  `stale_planning_evidence` before the generic execution-summary blocker
  return, mutating the change to `S1_PLAN/audit`.
- Recovery removes `verification/plan-audit.yaml`,
  `verification/wave-plan.yaml`, `verification/execution-summary.yaml`, and
  downstream run-summary-bound records:
  `verification/spec-compliance-review.yaml`,
  `verification/code-quality-review.yaml`,
  `verification/goal-verification.yaml`, and
  `verification/final-closeout.yaml`. It does not remove
  `.git/slipway/runtime/changes/<slug>` or
  `verification/wave-orchestration.yaml`.
- `cmd/next_skill_view.go` recognizes recoverable stale planning blockers and
  projects `run_slipway_run_to_advance` plus a recovery warning on read-only
  handoff surfaces.
- `internal/engine/gate.EvaluateGPivot` mirrors
  `cmd.validatePivotPreconditions`: reroute allowed from S1/S2/S3/S4, rescope
  allowed from S2 only.
- `internal/engine/progression.EvaluateGovernanceReadiness` adds S3/S4
  `scope_contract` guidance that tells operators to fix `tasks.md`
  `target_files` or execution scope first, then use stale-planning recovery
  when planning artifacts change. It does not point S3/S4 users toward
  `pivot --rescope`.

## Rollout and Rollback
Roll forward by adding focused tests first, implementing the recovery helper,
then updating JSON handoff and pivot gate parity. Roll back by reverting those
code/test changes; no persistent migration is introduced. Runtime recovery
side effects are limited to the active change's governed verification files
and are auditable through lifecycle events.

## Risk
- High: accidentally deleting runtime task evidence would make recovery
  destructive. Mitigation: tests assert task evidence and wave-orchestration
  verification survive recovery.
- High: stale plan-audit or wave-plan evidence could be reused. Mitigation:
  recovery removes those files before presenting `plan-audit`.
- High: old review/verify evidence could be reused after the execution summary
  is rebuilt with the same run summary version. Mitigation: recovery also
  removes downstream spec/code review, goal-verification, and final-closeout
  records and clears the corresponding evidence refs.
- Medium: preserved task evidence can be incompatible with changed `tasks.md`.
  Mitigation: existing wave sync task-plan drift blockers remain in the
  acceptance path.
- Medium: read-only `next` could advertise the wrong host. Mitigation: tests
  assert `run_slipway_run_to_advance` is present and recovery is the actionable
  route when stale planning blocks S3/S4.
- Medium: S3/S4 `scope_contract` failures could look like a `rescope` problem.
  Mitigation: readiness diagnostics explicitly explain the recovery path and
  that `pivot --rescope` remains S2-only.
