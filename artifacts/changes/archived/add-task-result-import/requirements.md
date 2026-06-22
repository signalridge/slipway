# Requirements

## Requirements

### Requirement: Compact Task Result Import
REQ-001: The system MUST support `slipway evidence task --result-file <json>
--json` as the documented executor-result import path for S2 task evidence and
MUST allow `--result-file` to be repeated for atomic multi-task batch import.

#### Scenario: Valid result file imports full task evidence
GIVEN an active governed change in S2 with a materialized wave plan containing
task `t-01`
WHEN an executor result JSON provides `task_id`, `verdict`, `evidence_ref`,
`changed_files`, `blockers`, and optional `session_id`
THEN Slipway writes runtime task evidence for `t-01` and reports the recorded
evidence path in JSON.

#### Scenario: Multiple result files import atomically
GIVEN an active governed change in S2 with a materialized wave plan containing
tasks `t-01` and `t-02`
WHEN the coordinator passes two `--result-file` values in one command
THEN Slipway validates every result file before writing, rejects duplicate task
IDs, writes evidence for both tasks on success, and writes no task evidence when
any member is invalid.

### Requirement: Slipway-Owned Ledger Fields
REQ-002: The system MUST derive `run_summary_version`, `task_kind`,
`target_files`, `captured_at`, and `freshness_inputs` from Slipway-owned state
instead of accepting those fields from executor result JSON.

#### Scenario: Executor cannot override internal ledger fields
GIVEN an executor result JSON for a valid task
WHEN the file includes `run_summary_version`, `task_kind`, `target_files`,
`captured_at`, `freshness_inputs`, or `input_hash`
THEN the import fails closed without writing task evidence and names the
unsupported field.

#### Scenario: Manual ledger flags cannot mix with result-file import
GIVEN an executor result JSON for a valid task
WHEN the command combines `--result-file` with any manual task ledger flag
THEN the import fails closed without writing task evidence and names the
conflicting flag.

### Requirement: Engine-Owned Run Boundary
REQ-003: The system MUST establish an engine-owned active execution
`run_summary_version` at the execution run boundary and use that version for
task evidence and wave-orchestration evidence.

#### Scenario: Fresh execution uses engine-owned version
GIVEN a change enters or refreshes S2 execution from the current task plan
WHEN task result JSON is imported for planned tasks
THEN every written task evidence record uses the active execution run version
chosen by Slipway, not a caller-supplied value.

### Requirement: Fail-Closed Version Ambiguity
REQ-004: The system MUST fail closed when existing task or wave evidence makes
the active run version ambiguous or contradictory.

#### Scenario: Mixed task evidence versions block import
GIVEN an active S2 change whose runtime task evidence directory contains task
records for more than one run version
WHEN an executor result JSON is imported
THEN Slipway rejects the import and reports remediation to clear or repair the
ambiguous execution evidence before recording more task evidence.

### Requirement: Scope Safety Preserved
REQ-005: The system MUST preserve per-task `changed_files` as executor-owned
input and continue enforcing existing changed-file scope and parallel-overlap
safety.

#### Scenario: Code task result without changed files blocks safety
GIVEN a planned task whose task kind requires changed-file attribution
WHEN a result file omits `changed_files`
THEN Slipway rejects or later blocks the evidence through the existing
scope-contract safety path rather than silently treating the task as safe.

### Requirement: Agent Guidance Uses Result Import
REQ-006: Generated agent-facing guidance MUST teach result JSON import as the
default task evidence path and MUST NOT require agents to select
`run_summary_version`, `task_kind`, or `target_files`.

#### Scenario: Wave orchestration guidance no longer teaches the long protocol
GIVEN generated wave-orchestration and evidence command guidance
WHEN an agent reads the task evidence instructions
THEN the instructions say to write compact result JSON and import it with
`slipway evidence task --result-file <path> --json`, repeating `--result-file`
when importing multiple task result files atomically.

### Requirement: Review-Driven Re-Execution Advances Version
REQ-007: The system MUST ensure fresh implementation re-execution after S3
review/fix work uses a newer engine-owned `run_summary_version` than the stale
execution it supersedes.

#### Scenario: S3 repair re-execution does not reuse stale run version
GIVEN a change has S3 review findings after execution run version 1
WHEN implementation work is re-executed for those findings
THEN the next task evidence import uses a new active run version and stale
review evidence cannot pass as if it belonged to the prior run.
