# Requirements

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -count=1 ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: S3S4PlanningRecoveryRoute
REQ-001: When a governed change in `S3_REVIEW` or `S4_VERIFY` has execution
freshness blocked by `stale_planning_evidence`, `slipway run` MUST provide a
non-destructive recovery transition back to a planning audit state instead of
leaving the operator at a dead-end instruction. Traces to INT-001.

#### Scenario: Review-state planning drift recovery
GIVEN a change is in `S3_REVIEW` with an existing execution summary
WHEN a planning artifact changes after the summary was captured
THEN `slipway run --json --diagnostics` transitions to `S1_PLAN/audit` with a
recovery reason and presents `plan-audit` as the next actionable host.

#### Scenario: Verify-state planning drift recovery
GIVEN a change is in `S4_VERIFY` with an existing execution summary
WHEN `tasks.md` or another planning artifact changes after execution evidence
THEN `slipway run --json --diagnostics` reopens planning audit without
finalizing or archiving the change.

### Requirement: ReadOnlyRecoveryGuidance
REQ-002: `slipway next --json` and diagnostic handoffs MUST expose an
actionable recovery route for recoverable S3/S4 stale planning evidence,
including `run_slipway_run_to_advance`, rather than surfacing only a stale
plan-audit instruction from a state that cannot call plan-audit. S3/S4
`scope_contract` failures MUST also guide the operator toward updating
`tasks.md` `target_files` or execution scope and then using stale-planning
recovery when planning artifacts change, not `pivot --rescope`. Traces to
INT-001.

#### Scenario: Query surface shows recovery action
GIVEN a change is blocked in S3/S4 by `stale_planning_evidence`
WHEN the operator runs `slipway next --json --diagnostics`
THEN the JSON output includes stale freshness diagnostics and a warning or
reason code directing the operator to run `slipway run` for recovery.

#### Scenario: Scope-contract drift points to recovery path
GIVEN a change is in S3/S4 and `scope_contract` fails because changed files do
not match the task target contract
WHEN `slipway validate --json` or `slipway next --json --diagnostics` renders
the blocked state
THEN diagnostics explain that the operator should fix `tasks.md` `target_files`
or execution scope first, and that planning changes recover through
`stale_planning_evidence` by running `slipway run` back to `S1_PLAN/audit`.

### Requirement: SelectiveEvidenceInvalidation
REQ-003: Stale-planning recovery MUST clear derived planning/execution
verification artifacts and downstream review/verify records that depend on the
stale planning inputs while preserving useful runtime execution evidence for
later rebuilds. Traces to INT-001.

#### Scenario: Recovery invalidates derived artifacts only
GIVEN stale planning recovery starts from S3/S4
WHEN the lifecycle reopens `S1_PLAN/audit`
THEN `verification/plan-audit.yaml`, `verification/wave-plan.yaml`, and
`verification/execution-summary.yaml` are removed if present. Downstream
run-summary-bound records `verification/spec-compliance-review.yaml`,
`verification/code-quality-review.yaml`, `verification/goal-verification.yaml`,
and `verification/final-closeout.yaml` are also removed if present, while
runtime task evidence under `.git/slipway/runtime/changes/<slug>/evidence/tasks`
and the existing `wave-orchestration.yaml` evidence are preserved.

#### Scenario: Old review evidence is not reused after rebuild
GIVEN stale-planning recovery started from S4 with passing review and
verification records for run summary version 1
WHEN task evidence is refreshed and `execution-summary.yaml` is rebuilt with
the same run summary version
THEN old spec/code review, goal-verification, and final-closeout records cannot
be reused; S3 requires fresh `spec-compliance-review` evidence.

### Requirement: RefreshOrderingAndFailClosedEvidence
REQ-004: Recovery MUST force the refreshed evidence chain through
plan-audit, wave-plan materialization, and execution-summary rebuild in that
order, and MUST remain fail-closed until the chain is complete. Traces to
INT-001.

#### Scenario: Refreshed chain order
GIVEN recovery has reopened `S1_PLAN/audit`
WHEN passing plan-audit evidence is recorded and `slipway run` advances
THEN Slipway materializes a fresh `wave-plan.yaml` before entering S2, and S2
wave synchronization rebuilds `execution-summary.yaml` from current wave/task
evidence.

#### Scenario: Incompatible task plan remains blocked
GIVEN recovered planning changed `tasks.md` semantically after task evidence
was captured
WHEN S2 wave synchronization runs
THEN the change remains blocked by task-plan drift instead of treating the
preserved execution evidence as fresh.

### Requirement: PivotPreconditionParity
REQ-005: `EvaluateGPivot` and CLI pivot preconditions MUST agree on allowed
pivot states: reroute is allowed from `S1_PLAN` and governed pivot states, and
rescope remains limited to `S2_EXECUTE` because the pivot path is destructive.
The CLI and gate surfaces SHOULD use `rescope_state_invalid` for the shared
rescope state failure. Traces to INT-001.

#### Scenario: Gate and CLI agree on rescope states
GIVEN candidate states S1, S2, S3, and S4
WHEN gate evaluation and CLI precondition checks are compared
THEN both allow `--rescope` only from S2 and reject S1/S3/S4 consistently.

### Requirement: RegressionAndGovernanceProof
REQ-006: The implementation MUST include focused regression tests for recovery
routing, recovery ordering, stale-evidence fail-closed behavior, downstream
review/verify invalidation, scope-contract recovery guidance, and pivot
precondition parity, plus full build/test/diff/governance proof before closeout.
Traces to INT-001.

#### Scenario: Verification commands pass
GIVEN implementation and test work is complete
WHEN verification is run
THEN focused tests, `go test -count=1 ./...`, `go build ./...`,
`git diff --check`, and `slipway validate --json` provide fresh passing proof.

### Requirement: ExternalAPIContractsGuardrail
REQ-007: JSON/CLI behavior changes MUST remain explicit, backward-compatible
where possible, and covered by domain-aware review evidence because this change
touches external API contracts. Traces to INT-001.

#### Scenario: Guardrail compliance
GIVEN `next`, `run`, `validate`, or `pivot` JSON output changes
WHEN the change is reviewed
THEN the domain review records the intentional contract changes and verifies
that failures remain actionable and fail-closed.
