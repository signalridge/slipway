## ADDED Requirements

### Requirement: Governance Skill Inventory
The system SHALL define governance contract skills aligned to states:
- `intake-analysis` (`S1_ANALYZE`)
- `scope-confirmation` (`S3_SCOPE_CONFIRMATION`, L3)
- `plan-audit` (`S5_PLAN_AUDIT`, governed lane)
- `wave-orchestration` (`S6_RUN_WAVES`)
- `artifact-review` (`S7_REVIEW`)
- `goal-verification` (`S8_VERIFY`)
- `final-closeout` (conditional governed `S8_VERIFY` closeout sub-step)

These SHALL remain distinct from command skills.

#### Scenario: Skill registry completeness
- **WHEN** governance skill registry is loaded
- **THEN** all seven governance skills SHALL exist

### Requirement: Mitigation Mapping
Each governance skill SHALL declare mitigation purpose:

- `intake-analysis`: unclear intent and hidden guardrail risk
- `scope-confirmation`: L3 discovery/scope drift
- `plan-audit`: stale or incomplete plan bundle
- `wave-orchestration`: uncontrolled parallel execution drift
- `artifact-review`: cross-artifact inconsistency
- `goal-verification`: false completion claims
- `final-closeout`: stale final evidence before governed ship decision

#### Scenario: Mitigation traceability
- **WHEN** governance evidence is emitted
- **THEN** mitigation mapping SHALL remain recoverable from `skill_name` (with optional denormalized `mitigation_target`)

### Requirement: Mitigation Mapping Consistency
Mitigation mapping SHALL be authoritative by `skill_name`.

`mitigation_target` field is optional denormalized metadata for audit readability.

Consistency contract:
- if `mitigation_target` is present, value SHALL match the registered mitigation mapping for emitting `skill_name`
- if `mitigation_target` is omitted, consumers SHALL derive mitigation mapping from skill registry without failing readiness
- mismatched `skill_name` and `mitigation_target` SHALL invalidate evidence for governance readiness

#### Scenario: Mitigation mismatch invalidates evidence
- **WHEN** evidence emits `skill_name=plan-audit` with mitigation target mapped to another skill
- **THEN** evidence SHALL be invalid for readiness checks until corrected

#### Scenario: Mitigation target omitted but mapping is derivable
- **WHEN** evidence omits `mitigation_target` and `skill_name` exists in registry
- **THEN** readiness checks SHALL derive mitigation mapping and continue without failure

### Requirement: Level Matrix by Governance Lane
Required governance skills SHALL be determined by level:

- `L1`: no mandatory governance skills by default in fixed-level mode
- `L2`: `intake-analysis`, `plan-audit`, `wave-orchestration`, `artifact-review`, `goal-verification`
- `L3`: L2 set + `scope-confirmation`
- `final-closeout` required only when governed `S8_VERIFY` detects stale/missing closeout evidence

For auto mode:
- `intake-analysis` SHALL always execute at `S1_ANALYZE` because final level is unresolved before analyze completes.

#### Scenario: L3 required skill set
- **WHEN** level is L3
- **THEN** required skills SHALL include `scope-confirmation`

#### Scenario: L1 has no mandatory governance skills
- **WHEN** level is L1
- **THEN** governance skills SHALL remain optional and SHALL NOT block progression

#### Scenario: Auto mode intake-analysis is mandatory
- **WHEN** operator runs `spln new --level auto`
- **THEN** `intake-analysis` evidence SHALL be required for `S1_ANALYZE` completion regardless of eventual routed level

### Requirement: State Binding
Skill-state bindings SHALL be:
- `intake-analysis` before leaving `S1`
- `scope-confirmation` before leaving `S3` (L3)
- `plan-audit` before leaving `S5` (governed)
- `wave-orchestration` during `S6`
- `artifact-review` before leaving `S7`
- `goal-verification` before leaving `S8`
- `final-closeout` during governed `S8` pre-ship closeout sub-step

#### Scenario: Missing required skill blocks transition
- **WHEN** required skill evidence is missing for current state
- **THEN** transition SHALL be blocked with explicit missing-skill reasons

### Requirement: Evidence Output Contract
Each governance skill execution SHALL write one JSON evidence file under `.spln/evidence/skills/`.

Required fields:
- `skill_name`
- `version`
- `run_summary_version`
- `session_id`
- `state`
- `verdict` (`pass|fail`)
- `blockers[]`
- `references[]`
- `timestamp`

Optional fields:
- `input_hash`
- `mitigation_target`
- `input_scope`
- `actor_id`
- `role`

`run_summary_version` value domain:
- pre-run-summary governance skills (`intake-analysis`, `scope-confirmation`, `plan-audit`) SHALL persist `run_summary_version=0`
- run-summary-bound governance skills (`wave-orchestration`, `artifact-review`, `goal-verification`, `final-closeout`) SHALL persist `run_summary_version>=1`
- review/verify readiness checks SHALL require reviewed evidence version to match the latest frozen run summary version

Conditional requiredness:
- run-summary-bound governance skills (`S6/S7/S8`) SHALL include non-empty `input_hash`
- pre-run-summary governance skills (`S1/S3/S5`) MAY omit `input_hash` in MVP

Evidence filename SHALL use deterministic, collision-safe pattern:
- `<session_id>--<skill_name>.json`

If the same filename already exists, writer SHALL append `--<n>` suffix (`n` starts at `1`) instead of overwriting.

Session identity contract:
- `session_id` SHALL be UUIDv7 lowercase canonical format
- one governance skill execution context uses exactly one `session_id`
- separate subagent/task executions SHALL use distinct `session_id` values

Actor identity contract:
- `actor_id` is optional in MVP and, when present, SHALL be stable actor identity across evidence/events for the same execution actor
- `role` is optional in MVP and, when present, SHALL be one of `implementer|reviewer|operator`
- reviewer-independence checks consume `session_id` as mandatory comparator identity; optional `role` remains audit metadata

Input hash contract:
- when present/required, `input_hash` SHALL be lowercase hex SHA-256 over canonical JSON input payload
- canonical payload SHALL include:
  - `request_id`
  - `state`
  - `run_summary_version`
  - normalized `input_scope[]`
  - relevant artifact/runtime fingerprints consumed by the skill
  - task/run identifiers when applicable
- canonicalization SHALL use sorted keys, UTF-8 encoding, and LF-normalized text
- semantically identical inputs SHALL produce identical `input_hash`

#### Scenario: Run-summary-bound evidence missing input hash
- **WHEN** run-summary-bound evidence (`S6/S7/S8`) lacks `input_hash`
- **THEN** evidence SHALL be invalid for governance readiness

#### Scenario: Evidence missing run summary version
- **WHEN** evidence lacks `run_summary_version` for run-summary-bound skills
- **THEN** evidence SHALL be invalid for governance readiness

#### Scenario: Pre-run-summary skill uses non-zero version
- **WHEN** `intake-analysis`, `scope-confirmation`, or `plan-audit` evidence uses `run_summary_version>0`
- **THEN** evidence SHALL be invalid for governance readiness

#### Scenario: Evidence missing mitigation target is still valid
- **WHEN** evidence lacks `mitigation_target` but `skill_name` is valid
- **THEN** readiness checks SHALL derive mitigation mapping from skill registry and continue

#### Scenario: Evidence filename collision
- **WHEN** evidence writer detects existing target filename
- **THEN** writer SHALL preserve existing file and create suffixed filename for new record

#### Scenario: Session id format is invalid
- **WHEN** evidence record uses non-UUIDv7 `session_id`
- **THEN** evidence SHALL be invalid for governance readiness

#### Scenario: Evidence role value is invalid
- **WHEN** evidence includes `role` and value is not one of `implementer|reviewer|operator`
- **THEN** evidence SHALL be invalid for governance readiness

#### Scenario: Canonical input hash is reproducible
- **WHEN** the same skill re-evaluates semantically identical canonical input payload
- **THEN** computed `input_hash` SHALL be identical

### Requirement: Gate Coupling
Gate checks SHALL consume governance evidence as follows:
- `G_scope` consumes `scope-confirmation`
- `G_plan` consumes `plan-audit`
- `G_ship` consumes required review/verification evidence for governed lane, including `final-closeout` evidence when governed `S8` closeout refresh is required

#### Scenario: Missing plan-audit blocks gate
- **WHEN** `G_plan` is evaluated without passing `plan-audit` evidence
- **THEN** `G_plan` SHALL be blocked

### Requirement: Reviewer Independence (Governed)
For L2/L3, approvals from `artifact-review` and `final-closeout` SHALL use reviewer identity different from primary implementer (different `session_id`).

Comparator contract:
- implementer baseline `session_id` SHALL come from latest `wave-orchestration` evidence for the same `(request_id, run_summary_version)`
- reviewer evidence (`artifact-review`, `final-closeout`) SHALL carry same `run_summary_version` as the reviewed frozen run summary
- reviewer evidence with identical `session_id` SHALL be invalid for governed readiness when comparing same `(request_id, run_summary_version)`
- missing implementer baseline SHALL block governed readiness with remediation to emit baseline evidence first

#### Scenario: L2 reviewer independence
- **WHEN** artifact-review runs for L2
- **THEN** reviewer `session_id` SHALL differ from implementer `session_id`

### Requirement: Technique Skills Non-Governance
Technique skills SHALL remain non-governance helpers and SHALL not be treated as gate controls.

Technique skills remain helper-only:
- `spln-tdd`
- `spln-systematic-debugging`
- `spln-code-review-protocol`

They SHALL:
- have `type: technique`
- not bind directly to gates
- not be mandatory gate evidence

#### Scenario: Technique output missing
- **WHEN** gate readiness is evaluated
- **THEN** missing technique skill output SHALL NOT block gates

### Requirement: Helper Trigger Guidance Is Advisory (MVP)
Helper trigger mapping SHALL be advisory UX metadata only and SHALL NOT be treated as a readiness dependency.

Guidance contract:
- helper hints MAY be derived from Superpowers/GSD/OpenSpec style catalogs when available
- missing local reference snapshots/files SHALL NOT block routing, review, verification, or gates
- helper hints SHALL NOT override required routing, skill, or gate contracts

#### Scenario: Missing helper reference snapshots does not block workflow
- **WHEN** helper source snapshots are unavailable in local workspace
- **THEN** workflow SHALL continue with core routing/gate/skill contracts unchanged

#### Scenario: Helper hints cannot override gate decisions
- **WHEN** advisory helper hint conflicts with required gate/skill contract
- **THEN** required gate/skill contract SHALL prevail and helper hint remains non-blocking
