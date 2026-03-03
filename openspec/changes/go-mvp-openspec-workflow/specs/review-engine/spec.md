## ADDED Requirements

### Requirement: Pragmatic Review Baseline
Review in MVP SHALL be behavior-driven and SHALL NOT require named review-layer taxonomy.

Baseline review behavior:
- structural completeness checks for governed artifacts
- implementation/spec alignment checks focused on changed/stale scope by default
- optional deep checks MAY run, but are not mandatory by default

#### Scenario: Structural review check
- **WHEN** review runs on governed artifacts
- **THEN** required sections and structural contracts SHALL be validated

### Requirement: Level-Controlled Review Baseline
Review baseline SHALL vary by level and stay minimal in MVP.

- L1: lightweight review via state checks; no governed review layers required
- L2/L3: minimum required baseline before ship is command checks + human review confirmation

#### Scenario: L2 baseline
- **WHEN** level is L2
- **THEN** review readiness SHALL depend on required command checks and `review_done=y`

### Requirement: Changed-Only Default
Review SHALL default to changed/stale scope.
`speclane review --all` SHALL force full review scope.

#### Scenario: Changed-only default
- **WHEN** `speclane review` runs without flags
- **THEN** only changed/stale units SHALL be reviewed

### Requirement: Review Fail Protocol
Review failures SHALL re-enter implementation loop for remediation.

When review finds blockers:
- return to `S6` for fix loop
- allow explicit `pivot` if intent drift persists

#### Scenario: Review blocker
- **WHEN** review finds unresolved blocker
- **THEN** workflow SHALL transition to `S6_RUN_WAVES`

### Requirement: Review Entry Preconditions
Review entry preconditions SHALL match CLI override-command contract.

- from `S7`: always allowed
- from `S6`: requires at least one frozen wave summary and no in-flight wave subprocess
- from `S8`: requires non-terminal request and at least one frozen wave summary

#### Scenario: Review precondition block
- **WHEN** review is requested from `S6` without frozen summary
- **THEN** review SHALL be precondition-blocked with deterministic remediation

### Requirement: Human Review Confirmation
Governed ship readiness SHALL require explicit human review confirmation:
- `review_done = y`

No reviewer `session_id` comparator is required by MVP.

#### Scenario: Missing human review confirmation
- **WHEN** review checks pass but `review_done` is missing or `n`
- **THEN** governed ship readiness SHALL remain blocked

### Requirement: Frozen Summary Consumption
Review SHALL consume latest frozen summary data from run record (`.speclane/runs/<request_id>.yaml`), not in-flight mutable task buffers.

#### Scenario: Review waits for frozen summary
- **WHEN** wave run is still in progress
- **THEN** review SHALL be blocked until summary is frozen in run record

### Requirement: Assurance Update Responsibility
Review/verify flow SHALL own `assurance.md` content updates for governed lanes.

Update timing:
- at `S7_REVIEW`: ensure `assurance.md` exists and update `Scope Summary`, `Evidence Index`, and `Residual Risks and Exceptions`
- at `S8_VERIFY`: update `Verification Verdict` and `Archive Decision` before done readiness evaluation

#### Scenario: Assurance updated during governed review
- **WHEN** governed review runs at `S7`
- **THEN** assurance sections owned by review SHALL be updated from current run evidence
