## ADDED Requirements

### Requirement: Gate Set
The system SHALL implement four gates:
- `G_scope`
- `G_plan`
- `G_pivot`
- `G_ship`

Gate status values:
- `pending`
- `approved`
- `blocked`

#### Scenario: Gate registry
- **WHEN** gate registry is loaded
- **THEN** only these four gate IDs SHALL exist

### Requirement: Lane Applicability
Gate applicability SHALL be level-dependent.

- L1 direct lane: no mandatory governed gates by default
- L2/L3 governed lanes: applicable gates are mandatory by state

#### Scenario: L1 direct execution
- **WHEN** level is L1 and no escalation happened
- **THEN** governed gates SHALL not block execution

### Requirement: G_scope (L3)
`G_scope` SHALL enforce L3 scope readiness before entering `S4`.

`G_scope` gates `S3 -> S4`.

Required check IDs and confirmations SHALL be resolved from `gate-checks/spec.md` gate-to-check mapping contract.

#### Scenario: Missing scope confirmation
- **WHEN** command checks pass but `scope_confirmed != y`
- **THEN** `G_scope` SHALL remain `blocked`

### Requirement: G_plan (L2/L3)
`G_plan` SHALL enforce governed plan readiness before entering `S6`.

`G_plan` gates `S5 -> S6`.

Required check IDs and confirmations SHALL be resolved from `gate-checks/spec.md` gate-to-check mapping contract.

#### Scenario: Validate failure blocks plan
- **WHEN** `openspec validate <change>` fails
- **THEN** `G_plan` SHALL be `blocked`

#### Scenario: Stale planning artifacts block plan
- **WHEN** `plan_artifacts_ready` check fails
- **THEN** `G_plan` SHALL be `blocked`

### Requirement: G_pivot
`G_pivot` SHALL constrain pivot entry and analyze-first semantics.

`G_pivot` applies to explicit pivot requests.

`G_pivot` is a rule gate in MVP (no catalog check IDs).

Rules:
- pivot entry states are `S6|S7|S8`
- pivot flow is analyze-first (`-> S1`) before reroute/rescope
- `rescope` is valid only from governed `S6`

#### Scenario: Invalid rescope entry
- **WHEN** `rescope` is requested from `S7` or `S8`
- **THEN** request SHALL be precondition-rejected

### Requirement: G_ship (L2/L3)
`G_ship` SHALL enforce ship readiness for governed lanes before `DONE`.

`G_ship` gates `S8 -> DONE` for governed lanes.

Required check IDs and confirmations SHALL be resolved from `gate-checks/spec.md` gate-to-check mapping contract.

Guardrail extension note:
- additional domain-specific ship checks are post-MVP and SHALL NOT be required in MVP gate decisions

`G_ship` SHALL NOT depend on governance-skill evidence file presence.

#### Scenario: Missing review confirmation blocks ship
- **WHEN** command checks pass but `review_done != y`
- **THEN** `G_ship` SHALL be `blocked`

### Requirement: Check Catalog Ownership Boundary
Gate engine SHALL define gate timing and pass/block semantics, but SHALL NOT define duplicate check catalogs.

Check-ID catalog and gate-to-check mapping are owned by `gate-checks/spec.md`.

#### Scenario: Single source check catalog
- **WHEN** gate definitions are reviewed
- **THEN** check IDs SHALL be referenced from `gate-checks/spec.md` rather than re-declared as independent lists

### Requirement: User Override on Failed Command Checks
User override flow SHALL be explicit and traceable in run record.

If a required command check fails, default behavior is `blocked`.

Operator MAY explicitly override and continue when all hold:
- failing check result is shown to operator
- operator gives explicit override confirmation (`y`)
- override trace is recorded on the overridden command check in run record (`override`, `override_note`, `override_at`)

Override scope:
- applies to mapped command checks in `G_scope`, `G_plan`, and `G_ship`
- does NOT apply to structural precondition rules of `G_pivot`

MVP override model is intentionally simple:
- no role hierarchy
- no dual approval
- no policy snapshot

#### Scenario: Operator overrides failed lint check
- **WHEN** lint check fails and operator confirms override
- **THEN** gate MAY transition to `approved` with override trace in run record

### Requirement: Required Gates by Level
Required gate set SHALL be derived from current level and pivot usage.

- `L3`: `G_scope + G_plan + G_ship` (+ `G_pivot` when pivoting)
- `L2`: `G_plan + G_ship` (+ `G_pivot` when pivoting)
- `L1`: none by default (+ `G_pivot` only when pivot path is used)

#### Scenario: L2 gate set
- **WHEN** level is L2 and not pivoting
- **THEN** required gates SHALL be `G_plan` and `G_ship`
