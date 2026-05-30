# Requirements
## Project Context
- Tech Stack: Go CLI
- Conventions: repo-native `go test ./...`, `go build ./...`, governed Slipway artifacts, structured JSON command surfaces
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Consistent actionable next skill across command surfaces
REQ-001: The system MUST make `slipway next`, `slipway validate`, and `slipway run --diagnostics` expose the same actionable next skill when a review-state blocker points at a missing skill.

#### Scenario: Spec review already passed and code quality is missing
GIVEN a governed change is in `S3_REVIEW`
AND `spec-compliance-review` evidence is present and passing
AND `code-quality-review` evidence is missing
WHEN an operator runs `slipway next --json`, `slipway validate --json`, or `slipway run --json --diagnostics`
THEN each JSON surface identifies `code-quality-review` as the actionable blocking next skill.
AND no surface presents an already-passing `spec-compliance-review` as the primary action.

#### Scenario: Display and blocking next skills differ intentionally
GIVEN a lifecycle step has both a conceptual display skill and a distinct blocking skill
WHEN diagnostics are requested
THEN the output exposes the display skill, blocking skill, and reason as separate structured fields.

### Requirement: Replace opaque execution hash with explicit structural freshness contract
REQ-002: The system MUST replace legacy `evidence_input_hash` task freshness with explicit stored structural input fields and field-by-field comparison. No compatibility layer is required for the old field.

#### Scenario: Fresh execution summary uses structural input fields
GIVEN execution evidence is generated for a task
WHEN `execution-summary.yaml` is written
THEN each task summary records the structural inputs used for freshness checks, including change id, run summary version, task id, and guardrail domain.
AND the task summary does not rely on `evidence_input_hash` as the freshness authority.

#### Scenario: Old hash-only summaries are stale by contract
GIVEN an existing task summary only contains `evidence_input_hash`
WHEN freshness diagnostics inspect the summary after this change
THEN the task is reported as stale or unsupported for the current contract.
AND the remediation tells the operator to regenerate execution evidence instead of silently accepting the old field.

### Requirement: Explain freshness failures with causality, values, timestamps, and remediation
REQ-003: The system MUST report stale artifact/evidence pairs, first stale cause, downstream evidence chain, expected/current values, relevant timestamps, and a safe next action in diagnostics for `validate`, `next --diagnostics`, `run --diagnostics`, `status`, `health`, and `repair` where those commands surface freshness.

#### Scenario: Planning evidence became stale after `tasks.md` changed
GIVEN a plan audit or wave plan was captured before the latest `tasks.md` update
WHEN diagnostics are requested
THEN the output names the stale evidence artifact, the newer source artifact, both timestamps when known, and the regeneration command or artifact to update.

#### Scenario: Stale evidence chain identifies the first stale cause
GIVEN an implementation adds a changed file after the original task plan was captured
AND `tasks.md` is updated after `wave-plan.yaml` and `execution-summary.yaml`
WHEN diagnostics are requested
THEN the output identifies `tasks.md` as the first stale source artifact.
AND it lists the downstream stale evidence chain, including affected plan-audit, wave-plan, and execution-summary evidence when present.
AND it provides the regeneration or rescope order so the operator does not repair downstream evidence before the source artifact is reconciled.

#### Scenario: Task structural inputs differ
GIVEN a task execution summary has structural freshness inputs that differ from the current expected inputs
WHEN diagnostics are requested
THEN the output lists each differing field with expected and current values.
AND the diagnostic identifies the evidence file that must be regenerated.

#### Scenario: Operator repair mistake is diagnosed directly
GIVEN an operator manually edits execution freshness fields such as `captured_at`, `tasks_plan_hash`, or task structural inputs to an incorrect value
WHEN diagnostics are requested
THEN the output identifies the edited field or timestamp that does not match the authoritative source artifact.
AND it reports expected and current values when known.
AND it tells the operator to regenerate or rescope the relevant evidence instead of continuing manual timestamp or hash edits.

### Requirement: Expose exact review layer tokens
REQ-004: The system MUST expose the exact review layer tokens consumed by governance gates and update generated skill templates so examples do not imply unsupported substitute tokens.

#### Scenario: Spec compliance review token requirements
GIVEN a governed change requires artifact review layers
WHEN `slipway next --json` or a spec-compliance skill prompt is generated
THEN the output names required tokens such as `layer:R0=pass` and any domain-required layer tokens such as `layer:R3=pass`.
AND it does not present `layer:CORRECTNESS=pass` or `layer:SAFETY=pass` as gate-satisfying substitutes.

#### Scenario: Code quality review token requirements
GIVEN a governed change requires implementation review layers
WHEN `slipway next --json` or a code-quality skill prompt is generated
THEN the output names required tokens such as `layer:IR1=pass` and any domain-required layer tokens such as `layer:IR3=pass`.

### Requirement: Explain `run --resume` lifecycle boundaries
REQ-005: The system MUST make `slipway run --resume` errors include current lifecycle state, resumable states, and the correct action for non-resumable states.

#### Scenario: Active change is in review state
GIVEN a governed change is active in `S3_REVIEW`
WHEN an operator runs `slipway run --resume --json`
THEN the error includes `current_state=S3_REVIEW`.
AND it lists the lifecycle states where resume applies.
AND the remediation directs the operator to normal run, validate, or review-evidence flow instead of implying that all active changes are resumable.

### Requirement: Surface linked-worktree authority paths
REQ-006: The system MUST expose path authority diagnostics that distinguish invocation workspace, bound worktree workspace, governed artifact bundle, git-common runtime evidence path, verification path, and change authority path.

#### Scenario: Linked worktree runtime evidence is stale or repaired
GIVEN a governed change is executed from a linked worktree under `.worktrees/<branch>`
WHEN stale runtime evidence or repaired runtime evidence is reported
THEN diagnostics include the exact authoritative git-common runtime evidence path for the current change.
AND the output distinguishes that path from the governed artifact bundle path.

### Requirement: Separate applied repairs from unrepaired drift
REQ-007: The system MUST make `repair --json` distinguish repairs that were applied from drift that was detected but not safely repaired.

#### Scenario: Repair recovers a wave run but leaves summary drift
GIVEN repair can recover runtime wave evidence
AND a planning or execution-summary drift remains unsafe to mutate automatically
WHEN `slipway repair --json` completes
THEN the output includes an `applied_repairs` entry for the recovered wave evidence.
AND it includes an `unrepaired_drift` entry with reason, evidence/artifact path, and next action.

### Requirement: Mark artifact DAG entries as blocking or informational
REQ-008: The system MUST make `status --json` artifact DAG entries say whether `draft` or `ready:false` is currently blocking a gate or only informational.

#### Scenario: Review state with non-blocking draft artifacts
GIVEN a change has reached `S3_REVIEW`
AND plan artifacts still show `state=draft` or `ready=false`
WHEN `slipway status --json` is requested
THEN each artifact DAG entry includes a current blocking flag and, when blocking, the gate or reason code.
AND non-blocking draft entries are labeled as non-blocking instead of appearing as silent blockers.

### Requirement: Mark active-change impact on health warnings
REQ-009: The system MUST identify whether health warnings, especially codebase-map warnings, block the active change.

#### Scenario: Codebase map warning does not block current governance gate
GIVEN `slipway health --json` reports stale or missing codebase-map files
AND the active change is blocked by unrelated review evidence
WHEN health output is generated
THEN the finding is marked non-blocking for the active change.
AND the output gives a concrete refresh command or target path if the map should be refreshed.

### Requirement: Clarify confirmation boundary semantics
REQ-010: The system MUST distinguish "must stop for user confirmation" from "may continue after recording evidence" in structured lifecycle diagnostics.

#### Scenario: Review skill handoff requires an explicit stop
GIVEN the next host skill requires a hard confirmation boundary
WHEN `next --json` or `run --diagnostics` returns the handoff
THEN the output includes structured confirmation requirements.
AND it states whether prior user authorization is sufficient or a fresh confirmation is required.

### Requirement: Regression tests and documentation/templates for changed CLI contracts
REQ-011: The system MUST include focused regression tests and documentation/templates for the changed command contracts.

#### Scenario: CLI contract tests cover issue #13 failures
GIVEN the implementation is complete
WHEN the targeted tests and full Go verification commands run
THEN they prove the next-skill, freshness causality, operator-mistake recovery, review-token, resume, path, repair, status, and health diagnostics required by this change.

### Requirement: external_api_contracts guardrail compliance
REQ-012: The implementation MUST comply with `external_api_contracts` guardrail requirements because this change alters structured CLI JSON outputs.

#### Scenario: JSON output contract is intentionally changed
GIVEN command JSON fields are added or replaced
WHEN implementation and documentation are reviewed
THEN changed fields are explicit, structured, tested, and documented.
