# Requirements

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Governance health exposes current traceability gap identities.
REQ-001: `slipway health --governance --json` MUST expose actionable traceability gap identities when `traceability_coherence` is WARN or FAIL. Traces to INT-001A.

#### Scenario: Failing traceability health names gaps
GIVEN a governed bundle whose traceability snapshot contains blocking gaps
WHEN `slipway health --governance --json --change <slug>` runs
THEN the `traceability_coherence` check identifies the offending gap IDs, types, issues, and blocking status.

#### Scenario: Current recompute behavior is preserved
GIVEN a persisted governance snapshot and a later artifact/control change
WHEN governance health runs
THEN current artifact/control state is recomputed or treated as the authoritative source, preserving existing material-change freshness behavior.

### Requirement: Handoff confirmation diagnostics name the correct action.
REQ-002: Skill-handoff confirmation output MUST explicitly distinguish non-checkpoint handoffs from active checkpoint resume and MUST not imply that `--resume-response` is valid unless an active checkpoint exists. Traces to INT-001B.

#### Scenario: Skill handoff is not checkpoint-resumable
GIVEN `next/run --json --diagnostics` returns a required skill handoff and no `input_context.resume_checkpoint`
WHEN `confirmation_requirement` is rendered
THEN it includes a non-checkpoint next action for the skill handoff and reports that `--resume-response` is not supported.

#### Scenario: Active checkpoint remains checkpoint-resumable
GIVEN a real `active_checkpoint` exists
WHEN `next/run --json --diagnostics` renders `confirmation_requirement`
THEN it reports that `--resume-response` is supported and keeps the checkpoint resume path intact.

### Requirement: Goal-verification placeholder scan is macOS-portable.
REQ-003: The generated `slipway-goal-verification` skill MUST NOT require GNU-only `grep -P` for the empty-block placeholder scan on macOS. Traces to INT-001C.

#### Scenario: Rendered skill uses portable scan
GIVEN the goal-verification skill template is rendered
WHEN its stub/placeholder scan section is inspected
THEN it does not contain `grep -P` and includes a portable empty-block scan command.

### Requirement: Scope remains limited to the three open issues.
REQ-004: The implementation MUST stay scoped to the Slipway CLI/governance-health/progression/template/docs behavior required for issues #59, #61, and #62. Traces to INT-001D.

#### Scenario: No unrelated governance redesign
GIVEN the implementation diff
WHEN reviewed against the change intent
THEN it modifies only the health diagnostics, confirmation diagnostics, planning evidence freshness guard, goal-verification template, command contract docs, and their tests unless a directly required helper is introduced.

### Requirement: external_api_contracts guardrail compliance.
REQ-005: Additive JSON-surface changes MUST be covered by regression tests and MUST preserve existing fields. Traces to INT-001.

#### Scenario: Additive contract change
GIVEN existing JSON consumers rely on current `confirmation_requirement` and governance health fields
WHEN this change adds action/detail metadata
THEN current field names and meanings remain available and tests cover the new metadata.

### Requirement: Runtime task evidence must be fresh.
REQ-006: A passing `wave-orchestration` record MUST NOT satisfy a gate when runtime task evidence for the same run summary version is newer than that record. Traces to INT-001E.

#### Scenario: New task evidence invalidates prior wave orchestration
GIVEN `verification/wave-orchestration.yaml` is passing for a run summary version
AND runtime task evidence for that same run summary version has a newer `captured_at`
WHEN wave execution is synchronized
THEN the old wave orchestration record is not reused and sync reports `wave_orchestration_stale_task_evidence:<task_id>`.

> **Descoped — plan-audit source-freshness moved to #66.** An earlier revision of
> this requirement also failed `plan-audit` evidence closed when a planning source
> artifact (`intent/requirements/research/decision/tasks.md`) had a newer
> filesystem mtime than the verification record. That guard was removed before
> ship: Slipway itself rewrites `tasks.md` during execution (checkbox writeback in
> `syncCompletedTaskCheckboxes`), so an mtime/timestamp comparison false-positives
> on a normal `[ ]`→`[x]` tick and would report `plan-audit:stale_evidence` on
> read-only surfaces after S2. Source-fresh plan-audit evidence is re-approached in
> #66 via a content digest (`TaskPlanSemanticHash`, checkbox-invariant) bound at
> acceptance, not wall-clock time. Only the wave-orchestration half — which
> compares the embedded logical `captured_at`, never mtime — remains in this change.

### Requirement: Command surfaces explain their authority boundary.
REQ-007: Command documentation and generated command prompts MUST distinguish active readiness (`validate`), mutating transition result (`run`), and diagnostic health feedback (`health --governance`) so error-severity blockers after a successful `run` advance are not misread as command failure. Traces to INT-001F.

#### Scenario: Run transition and next blockers are distinct
GIVEN `slipway run --json` advances and then stops at a required skill
WHEN an operator reads the command contract
THEN `advanced` is documented as the invocation's mutation result and `blockers` is documented as the current stop condition after that mutation.

#### Scenario: Read-only gate projections agree after planning
GIVEN planning evidence passed before the change reached verification
WHEN status, validate, and next project gate status after closeout assurance edits
THEN all three surfaces keep the planning gate approved and none downgrades it.
