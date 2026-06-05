# Requirements

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: ReadOnlyS2SpecificTaskEvidenceDiagnostics
REQ-001: When S2 has passing `wave-orchestration` evidence and present task
evidence that is stale, invalid, non-passing, or mismatched against the current
task plan, read-only surfaces MUST report the specific task-evidence blocker
instead of collapsing the state to `wave-orchestration:run_summary_missing`.
Traces to issue #72.

#### Scenario: Plan-drifted task evidence is specific
GIVEN a governed change is in `S2_EXECUTE` with passing wave evidence, task
evidence for run summary version 1, no `execution-summary.yaml`, and a
semantically changed `tasks.md`
WHEN `slipway next --json --diagnostics`, `slipway validate --json`, or
`slipway status --json` is run
THEN blockers include `tasks_plan_changed_since_task_evidence:<task-id>` and
do not include `wave-orchestration:run_summary_missing`.

### Requirement: MissingEvidencePathPreserved
REQ-002: When S2 has passing `wave-orchestration` evidence but no task evidence
for the run summary version, read-only and mutating surfaces MUST keep the
missing task-evidence guidance path, including the task evidence directory and
required fields. Traces to issue #72 non-goal.

#### Scenario: Absent task evidence stays actionable
GIVEN a governed change is in `S2_EXECUTE` with passing wave evidence and no
matching task evidence
WHEN readiness is rendered
THEN the blocker remains actionable as missing task evidence rather than being
reclassified as task-plan drift.

### Requirement: ReadOnlySurfacesRemainQueryOnly
REQ-003: `next`, `validate`, and `status` readiness evaluation MUST NOT write
`execution-summary.yaml`, wave-run files, or task checklist completion state
while refining S2 diagnostics. Traces to the command contract.

#### Scenario: Diagnostic preview does not materialize summary
GIVEN the S2 plan-drift fixture has no `execution-summary.yaml`
WHEN read-only command surfaces are executed
THEN no `execution-summary.yaml` is created as a side effect.

### Requirement: SharedDiagnosisNoCommandDuplication
REQ-004: The implementation SHOULD reuse the execution-path wave/task evidence
diagnosis so future blocker behavior remains aligned between `run` and
read-only surfaces. It MUST NOT duplicate divergent diagnosis logic in each
command renderer.

#### Scenario: Mutating sync behavior is unchanged
GIVEN existing wave-sync tests for stale task evidence, parse issues, plan
drift, and successful summary materialization
WHEN the implementation is complete
THEN those tests continue to pass.

### Requirement: RegressionAndGovernanceProof
REQ-005: The change MUST include focused regressions for the affected command
surfaces and focused progression tests, plus fresh full build/test/diff and
governance proof before closeout. Traces to issue #72.

#### Scenario: Verification commands pass
GIVEN implementation is complete
WHEN verification runs
THEN focused command tests, focused progression tests, `go test ./...`,
`go build ./...`, `git diff --check`, and `slipway validate --json` provide
fresh passing evidence.

### Requirement: ExternalAPIContractsGuardrail
REQ-006: Because JSON blockers are externally consumed command output, the
intentional contract change MUST be reviewed as `external_api_contracts` and
must remain backward-compatible for absent task evidence.

#### Scenario: Domain review covers JSON blocker change
GIVEN `validate`, `status`, and `next` JSON blockers change for present
stale task evidence
WHEN review evidence is recorded
THEN the review states that the changed blocker is intentional, actionable,
and fail-closed.
