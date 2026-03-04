## ADDED Requirements

### Requirement: Context Pack Generation
The system SHALL generate compact agent-consumable context per action containing:
1. intent summary
2. relevant units/files and minimal dependencies
3. unresolved blockers
4. recent decisions and next action

Long logs SHALL be referenced via `.spln/evidence/*` paths, not inlined.

#### Scenario: Context pack content
- **WHEN** `spln context` is run during execution
- **THEN** output SHALL include intent, changed scope, blockers, and current/next action

### Requirement: Dual-Lane Context Support
`spln context` SHALL support both lane types:
- admission/direct lane (`L1`, no governed change required)
- governed lane (`L2/L3`, change-linked)

Context SHALL explicitly include lane mode and source state file.

#### Scenario: Admission-only context
- **WHEN** active request is L1 direct lane
- **THEN** context SHALL identify admission source and SHALL NOT require change artifact metadata

### Requirement: Diagnostics Context Mode
When no unique active request context is resolvable (`0` or `>1` active requests), `spln context` SHALL still return compact diagnostics context.

Diagnostics-mode output SHALL:
- set `lane_mode=diagnostics`
- set `evidence_freshness=unknown`
- omit request-scoped progression fields that require unique active binding
- include actionable remediation for recovering deterministic active context

#### Scenario: Context runs in diagnostics mode
- **WHEN** active set size is `0` or `>1`
- **THEN** `spln context` SHALL return diagnostics context instead of failing request-scoped resolution

### Requirement: GSD-Aligned Wave Envelope
For `S6_RUN_WAVES`, context packs SHALL include:
- `wave_id`
- `task_id`
- `depends_on[]`
- `target_files[]`
- `task_kind`
- `autonomous`
- `checkpoint_type` (if any)
- `must_haves` (goal-backward verification anchors)

#### Scenario: Wave task envelope
- **WHEN** wave task context is generated
- **THEN** envelope fields SHALL be present

### Requirement: `spln context` Command Contract
`spln context` output SHALL be compact (<50 lines typical) and distinct from `spln status` (default JSON full-fidelity payload).

Minimum fields:
- lane mode
- level, `level_source`
- current state/action
- next-ready actions
- blockers
- evidence freshness summary

`next-ready actions` and `blockers` in context output SHALL be runtime projections derived from latest lane state and gate/artifact/task conditions.

#### Scenario: Context vs status distinction
- **WHEN** `spln context` and `spln status` are both run
- **THEN** context SHALL be concise and status SHALL remain full-fidelity

### Requirement: Multi-Format Output
`spln context` SHALL support `--format text|yaml|json` (default text).

#### Scenario: YAML output
- **WHEN** `spln context --format yaml` is run
- **THEN** output SHALL be valid YAML with all required context fields

### Requirement: Subagent Context Injection
When spawning subagents for wave execution, system SHALL:
1. inject task-scoped context
2. include relevant technique skill references
3. exclude unrelated artifacts/evidence
4. ensure each subagent uses unique `session_id` in UUIDv7 lowercase canonical format

#### Scenario: Scoped subagent input
- **WHEN** subagent runs one wave task
- **THEN** context SHALL be limited to task scope and required contracts

### Requirement: Checkpoint Resume Bundle
On checkpoint resume, context engine SHALL emit continuation bundle with:
- prior run id
- paused task id
- checkpoint type
- user response payload
- blockers at pause time

#### Scenario: Resume bundle integrity
- **WHEN** paused task resumes
- **THEN** continuation bundle SHALL be included and referenced in evidence

### Requirement: Evidence Freshness Tracking
Context engine SHALL report evidence freshness using:
- `input_hash` compatibility with current artifact/task versions
- timestamp ordering vs latest relevant updates

Evidence freshness value domain SHALL be:
- `fresh`: all required evidence references are current for active context
- `stale`: one or more required evidence references are outdated
- `unknown`: no active request context or insufficient inputs for evaluation

#### Scenario: Stale evidence detection
- **WHEN** inputs changed after evidence collection
- **THEN** context SHALL mark affected evidence as stale

#### Scenario: Unknown freshness without active context
- **WHEN** no unique active request context is resolvable
- **THEN** context/status freshness output SHALL be `unknown`
