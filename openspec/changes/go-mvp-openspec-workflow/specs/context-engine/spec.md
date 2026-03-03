## ADDED Requirements

### Requirement: Compact Context Pack
`speclane context` SHALL produce compact action-ready context including:
1. intent summary
2. relevant scope/files
3. blockers
4. next action
5. recent check results and human confirmations

Long outputs SHALL be referenced, not inlined.

#### Scenario: Context content
- **WHEN** context is requested during execution
- **THEN** output SHALL include intent, blockers, and next action

### Requirement: Dual-Lane Context Support
Context SHALL support:
- admission/direct lane (`L1`)
- governed lane (`L2/L3`)

#### Scenario: Admission-only context
- **WHEN** active request is L1
- **THEN** context SHALL not require governed change metadata

### Requirement: Diagnostics Mode
When active context is unresolved (`0` or `>1` active requests), context SHALL return diagnostics mode:
- `lane_mode=diagnostics`
- include remediation hints
- omit request-scoped progression fields

#### Scenario: Diagnostics context
- **WHEN** active context is ambiguous or missing
- **THEN** command SHALL return diagnostics context instead of hard fail

### Requirement: Wave Envelope
For `S6_RUN_WAVES`, context SHALL include:
- `wave_id`
- `task_id`
- `depends_on[]`
- `target_files[]`
- `task_kind`
- checkpoint metadata (when applicable)

#### Scenario: Wave task context
- **WHEN** wave task is executed
- **THEN** envelope fields SHALL be present

### Requirement: Multi-Format Output
`context` SHALL support `--format text|yaml|json` (default text).

#### Scenario: JSON format
- **WHEN** `speclane context --format json` is used
- **THEN** output SHALL remain parse-stable and compact

### Requirement: Confirmation-Aware Context
Context SHALL include latest confirmation states relevant to gates:
- `scope_confirmed` (L3)
- `execute_ready` (governed planning)
- `review_done`
- `ship_ready`

#### Scenario: Missing ship confirmation
- **WHEN** governed flow reaches `S8` and `ship_ready` is absent or `n`
- **THEN** context SHALL show ship as blocked by human confirmation
