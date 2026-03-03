## ADDED Requirements

### Requirement: Canonical State Taxonomy
Workflow SHALL use canonical states:
- `S0_INTAKE`
- `S1_ANALYZE`
- `S2_DISCOVER`
- `S3_SCOPE_CONFIRMATION`
- `S4_SPEC_BUNDLE`
- `S5_PLAN_AUDIT`
- `S6_RUN_WAVES`
- `S7_REVIEW`
- `S8_VERIFY`
- `DONE`

Cancellation is lifecycle status (`cancelled`), not an extra state ID.

#### Scenario: Cancellation uses lifecycle status
- **WHEN** request is cancelled
- **THEN** lifecycle status SHALL become `cancelled` without adding new state IDs

### Requirement: Two-Phase Workflow
Workflow SHALL execute admission first, then level-specific execution path.

- Admission phase: `S0 -> S1`
- Execution phase by level:
  - `L1`: `S6 -> S7 -> S8 -> DONE`
  - `L2`: `S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
  - `L3`: `S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

#### Scenario: Admission runs first
- **WHEN** `speclane new` starts
- **THEN** `S0` and `S1` SHALL complete before execution path selection

### Requirement: Post-`speclane new` Landing State
For executable requests, `speclane new` SHALL persist first execution state:
- `L1 -> S6_RUN_WAVES`
- `L2 -> S4_SPEC_BUNDLE`
- `L3 -> S2_DISCOVER`

#### Scenario: L3 lands at discovery
- **WHEN** route is `L3`
- **THEN** first `speclane do` SHALL execute `S2_DISCOVER`

### Requirement: `non_speclane` Rejection
Non-executable intake SHALL be rejected from workflow execution.

If intake is pure Q&A/advisory or clarification-required non-executable:
- classify as `non_speclane`
- do not create `request_id`
- do not persist runtime request state

#### Scenario: Pure Q&A rejected
- **WHEN** `S1_ANALYZE` classifies intake as non-executable
- **THEN** workflow SHALL terminate before any level path

### Requirement: Level Path Mapping
Path mapping SHALL be:
- `L1`: `S0 -> S1 -> S6 -> S7 -> S8 -> DONE`
- `L2`: `S0 -> S1 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
- `L3`: `S0 -> S1 -> S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

#### Scenario: L2/L3 share governed mainline
- **WHEN** state reaches `S4` in L2/L3
- **THEN** remaining path SHALL be identical (`S4..S8`)

### Requirement: L1 Lightweight Review/Verify
L1 lightweight review/verify path SHALL remain non-governed by default.

L1 keeps `S7/S8` as lightweight checks:
- no governed gate dependency
- no governance-skill evidence requirement
- `S8 -> DONE` still requires explicit `speclane done`

L1 `S7_REVIEW` behavior:
- evaluate latest frozen wave summary
- pass only when there are no unresolved non-pass tasks or open blockers
- on failure, transition `S7 -> S6` for remediation

L1 `S8_VERIFY` behavior:
- run lightweight verification on latest execution outputs
- execute task-level `verify_cmd` only when present
- mark request done-ready only when all required lightweight verification checks pass

#### Scenario: L1 done is command-gated
- **WHEN** L1 passes lightweight checks
- **THEN** state MAY be done-ready, but `DONE` requires explicit `speclane done`

#### Scenario: L1 review failure returns to execution
- **WHEN** L1 review finds unresolved non-pass tasks or blockers
- **THEN** workflow SHALL transition from `S7_REVIEW` back to `S6_RUN_WAVES`

### Requirement: Governed Scope/Plan/Ship Execution
Governed levels SHALL use minimal gate checks:
- `S3` uses `G_scope` (L3 only)
- `S5` uses `G_plan` (L2/L3)
- `S8` uses `G_ship` (L2/L3)

`G_ship` input is command checks + human confirmations, not governance-skill file presence.

#### Scenario: Governed S8 requires checks + confirmations
- **WHEN** L2/L3 executes `S8_VERIFY`
- **THEN** workflow SHALL evaluate required command checks and collect review/ship confirmations before `S8 -> DONE`

### Requirement: Transition and Loop Rules
Workflow engine SHALL enforce deterministic main transitions and loop transitions.

Main transitions:
- `S0 -> S1`
- `S1 -> S6` (L1)
- `S1 -> S4` (L2)
- `S1 -> S2` (L3)
- `S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

Loop transitions:
- `S5 -> S4` on plan readiness failure
- `S6 -> S6` for retry
- `S7 -> S6` when review fails
- `S8 -> S6` when verify fails
- `S6/S7/S8 -> S1` only through explicit `pivot`/`analyze` flows

#### Scenario: Plan remediation loop
- **WHEN** plan readiness fails at `S5`
- **THEN** workflow SHALL return to `S4`

### Requirement: State Ownership
State ownership SHALL be split across admission, governed, and run-record files.

- admission/direct state: `.speclane/runtime/admissions/<request_id>.yaml`
- governed state: `.speclane/runtime/changes/<request_id>.yaml`
- run/check record: `.speclane/runs/<request_id>.yaml` (execution ledger only; not authoritative lifecycle state source)

#### Scenario: Governed handoff
- **WHEN** route requires governed lane
- **THEN** admission SHALL be sealed and governed state SHALL become mutable execution source
