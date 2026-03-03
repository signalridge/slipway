# gate-engine Specification

## Purpose
TBD - created by archiving change go-mvp-openspec-workflow. Update Purpose after archive.
## Requirements
### Requirement: Gate Set
The system SHALL implement exactly four gates:
- `G_scope`
- `G_plan`
- `G_pivot`
- `G_ship`

Gate status SHALL be one of:
- `pending`
- `approved`
- `blocked`

#### Scenario: Gate registry inventory
- **WHEN** gate registry is loaded
- **THEN** it SHALL contain only `G_scope`, `G_plan`, `G_pivot`, `G_ship`

### Requirement: Gate Applicability by Lane
Gates are governance controls and SHALL apply only to governed lane (`L2/L3`) by default.

- L1 direct lane SHALL not require mandatory gate approvals
- `G_pivot` MAY apply when pivoting across control boundaries

#### Scenario: L1 direct lane
- **WHEN** level is L1 and no escalation is requested
- **THEN** mandatory gate checks SHALL NOT block direct-lane progression

### Requirement: G_scope Semantics (L3)
`G_scope` SHALL gate `S3_SCOPE_CONFIRMATION -> S4_SPEC_BUNDLE`.

Approval requires:
- discovery output exists
- `explore.md` exists for L3 governed changes
- `explore.md` includes MVP minimum sections with at least one non-empty item each:
  - `Objectives`
  - `Unknowns`
  - `Assumptions`
  - `Scope Boundaries`
  - `Validation Plan`
- scope confirmation evidence exists
- dedicated worktree metadata exists:
  - `worktree_path`
  - `worktree_branch`
- dedicated worktree metadata authenticity is verified:
  - `worktree_path` exists and is accessible
  - `worktree_path` is a registered Git worktree of current repository
  - checked-out branch at `worktree_path` matches persisted `worktree_branch`

Metadata source contract:
- `worktree_path` and `worktree_branch` SHALL be loaded from governed runtime change state persisted during `S3_SCOPE_CONFIRMATION`

#### Scenario: Missing worktree metadata
- **WHEN** level is L3 and `worktree_path` or `worktree_branch` is missing
- **THEN** `G_scope` SHALL be `blocked` with reason `dedicated worktree metadata required`

#### Scenario: Non-existent worktree path blocks scope gate
- **WHEN** level is L3 and persisted `worktree_path` does not exist or is not accessible
- **THEN** `G_scope` SHALL be `blocked` with reason `dedicated worktree path is invalid`

#### Scenario: Worktree branch mismatch blocks scope gate
- **WHEN** level is L3 and actual checked-out branch at `worktree_path` differs from persisted `worktree_branch`
- **THEN** `G_scope` SHALL be `blocked` with reason `dedicated worktree branch mismatch`

#### Scenario: Missing explore artifact blocks scope gate
- **WHEN** level is L3 and `explore.md` is missing
- **THEN** `G_scope` SHALL be `blocked` with reason `explore.md required for L3 scope gate`

#### Scenario: Incomplete explore structure blocks scope gate
- **WHEN** level is L3 and `explore.md` misses any required MVP section or has empty section content
- **THEN** `G_scope` SHALL be `blocked` with actionable remediation listing missing sections

### Requirement: G_plan Semantics (L2/L3)
`G_plan` SHALL gate `S5_PLAN_AUDIT -> S6_RUN_WAVES`.

Approval requires:
- required governed planning artifacts are present and non-stale
- plan-audit evidence is passing
- pre-run checks pass

#### Scenario: Stale spec bundle blocks plan gate
- **WHEN** any required governed planning artifact is stale
- **THEN** `G_plan` SHALL be `blocked` with stale-artifact reasons

### Requirement: G_pivot Semantics
`G_pivot` SHALL be evaluated on pivot/rescope requests.

Applicability boundary:
- valid pivot entry states are `S6_RUN_WAVES`, `S7_REVIEW`, `S8_VERIFY`
- rescope-specific entry state is governed `S6_RUN_WAVES` only
- requests from other states SHALL be rejected by command-layer preconditions before gate approval logic runs

Pivot request kinds:
- `reroute`: analyze result changes effective level/path (for example `L1 -> L2`, `L3 -> L2`)
- `rescope`: effective level remains unchanged in governed lane, but scope/contracts must be refreshed:
  - `L2`: `S6 -> S4`
  - `L3`: `S6 -> S3` (then `G_scope` re-evaluation before `S4`)

Evaluation sequence:
1. transition pivot flow to `S1_ANALYZE` and refresh analyze evidence
2. evaluate `G_pivot` against refreshed analyze evidence + explicit pivot kind
3. apply reroute/rescope transition only when `G_pivot` is approved

Approval requires:
- common:
  - explicit operator pivot intent (`reroute` or `rescope`)
  - fresh analyze evidence bound to current request context
- for `reroute`:
  - updated route rationale from refreshed `route_snapshot`
  - updated level metadata/history when level changes
  - target-lane required artifacts/gates become satisfiable
- for `rescope` (same governed level):
  - unchanged governed level proof in analyze result
  - explicit rescope rationale and scope delta
  - governed target-state refresh plan recorded:
    - `L2`: `S4_SPEC_BUNDLE` artifacts marked for refresh
    - `L3`: `S3_SCOPE_CONFIRMATION` scope/worktree revalidation plus subsequent `S4_SPEC_BUNDLE` refresh

#### Scenario: Missing reroute evidence
- **WHEN** pivot is requested without updated route evidence
- **THEN** `G_pivot` SHALL be `blocked`

#### Scenario: Governed rescope requires G_pivot approval
- **WHEN** operator requests rescope from `S6` with unchanged governed level
- **THEN** `G_pivot` SHALL still be evaluated and MUST approve before governed rescope transition (`L2: S6->S4`, `L3: S6->S3`)

#### Scenario: Governed rescope outside S6 is precondition-rejected
- **WHEN** operator requests `rescope` while current state is `S7_REVIEW` or `S8_VERIFY`
- **THEN** command-layer preconditions SHALL reject request before `G_pivot` approval path runs

### Requirement: G_ship Semantics (Governed Completion)
`G_ship` SHALL gate governed transition `S8_VERIFY -> DONE`.

`G_ship` SHALL evaluate outputs from governed `S8` sub-steps (`goal-verification` and conditional `final-closeout`).

Ship-stage required governed artifact set is fixed in MVP:
- `change.yaml`
- `proposal.md`
- `spec.md`
- `design.md`
- `tasks.md`
- `assurance.md`

`explore.md` is an L3 scope-stage artifact and SHALL be validated by `S2/S3 + G_scope`; it is out of ship-stage required-approval set.

`G_ship` SHALL block when any is true:
1. required ship-stage governed artifact is missing, unapproved, or structurally invalid
2. unresolved blockers exist
3. required governed `S8` verification evidence for current frozen run summary is missing or failing:
   - `goal-verification` always required
   - `final-closeout` required only when closeout refresh is requested by `S8`
4. required high-risk checks are missing or failing for active guardrail domain

Simplification boundary:
- stale-artifact classification, level-metadata integrity, and detailed evidence-shape validation are enforced by state/review contracts and surfaced into `G_ship` as blockers/evidence failures
- `G_ship` remains the final readiness gate, not a duplicate validator for every upstream invariant

Manifest validation boundary for `change.yaml` in MVP:
- `change.yaml` is a lightweight manifest and SHALL be validated as `R0` structural/identifier integrity only
- `G_ship` SHALL require `change.yaml` presence + valid identifiers/structure, but SHALL NOT require content-quality review layers (`R1/R2/R3`) for `change.yaml`

#### Scenario: Missing required evidence blocks ship
- **WHEN** governed lane is missing required governance evidence
- **THEN** `G_ship` SHALL be `blocked` with explicit missing evidence reasons

#### Scenario: Missing required S8 closeout evidence blocks ship
- **WHEN** governed `S8_VERIFY` requires closeout refresh and `final-closeout` evidence is missing or stale
- **THEN** `G_ship` SHALL remain `blocked` until refreshed closeout evidence is available

#### Scenario: Runtime metadata issue blocks ship via surfaced blocker
- **WHEN** state/review integrity checks detect governed runtime metadata inconsistency
- **THEN** `G_ship` SHALL block through surfaced blocker/evidence failure without introducing duplicate gate-specific metadata rules

#### Scenario: Ship pass
- **WHEN** all ship conditions are satisfied in governed lane
- **THEN** `G_ship` SHALL be `approved`

#### Scenario: L3 ship does not require explore approval
- **WHEN** level is L3 and `explore.md` has passed scope-stage checks at `S2/S3 + G_scope`
- **THEN** ship-stage `G_ship` SHALL evaluate the fixed ship-stage artifact set and SHALL NOT require additional `explore.md` approval

### Requirement: Guardrail High-Risk Check Catalog
High-risk checks SHALL be deterministic and guardrail-domain-specific.

Catalog by canonical `guardrail_domain` (`domain_slug`):
- `auth_authz`:
  - `auth_authz.safety_baseline`: authorization and deny-by-default baseline checks pass
- `security_credentials`:
  - `security_credentials.safety_baseline`: secret-handling and redaction baseline checks pass
- `privacy_pii`:
  - `privacy_pii.safety_baseline`: minimization/redaction and retention-boundary baseline checks pass
- `financial_flows`:
  - `financial_flows.safety_baseline`: idempotency and ledger-integrity baseline checks pass
- `schema_data_migration`:
  - `schema_data_migration.safety_baseline`: compatibility and rollback-viability baseline checks pass
- `irreversible_operations`:
  - `irreversible_operations.safety_baseline`: backup/undo readiness and explicit-confirmation baseline checks pass
- `external_api_contracts`:
  - `external_api_contracts.safety_baseline`: compatibility baseline checks pass (no unapproved breaking contract change)

Check ID naming contract:
- registry IDs above are the only valid `check_id` values in MVP
- format SHALL be `<domain_slug>.<check_slug>` with lowercase snake_case segments
- `high_risk_check_missing:<check_id>` and `high_risk_check_failed:<check_id>` MUST use a registry-defined `check_id`

Optional extension boundary:
- finer-grained domain checks MAY be recorded as additional evidence
- additional checks SHALL be non-blocking by default unless policy explicitly escalates required set

Evaluation contract:
- when `guardrail_domain` is empty, catalog checks are not required by `G_ship`
- when `guardrail_domain` is present, required domain baseline check(s) SHALL be evaluated before `G_ship` can be `approved`
- missing check evidence SHALL be emitted as `high_risk_check_missing:<check_id>`
- failing check evidence SHALL be emitted as `high_risk_check_failed:<check_id>`

Evidence source contract:
- high-risk check outcomes SHALL be backed by governed review/verification evidence (`IR3`, `goal-verification`, conditional `final-closeout`)
- `assurance.md` Evidence Index SHALL reference the corresponding check evidence for auditability

#### Scenario: Guardrail ship blocked by missing high-risk check evidence
- **WHEN** guardrail domain exists and one required high-risk check has no evidence
- **THEN** `G_ship` SHALL be `blocked` with `high_risk_check_missing:<check_id>` reason

#### Scenario: Guardrail ship blocked by failing high-risk check
- **WHEN** guardrail domain exists and one required high-risk check fails
- **THEN** `G_ship` SHALL be `blocked` with `high_risk_check_failed:<check_id>` reason

### Requirement: Required Gates by Level
The gate engine SHALL enforce required gate sets by level as listed below.

Required gate sets:
- `L3`: `G_scope + G_plan + G_ship` (+`G_pivot` when pivoting)
- `L2`: `G_plan + G_ship` (+`G_pivot` when pivoting)
- `L1`: none by default (+`G_pivot` when escalation/pivot path requires)

#### Scenario: L2 gate set
- **WHEN** level is L2 with no pivot request
- **THEN** required gates SHALL be `G_plan` and `G_ship`

### Requirement: Human Decision Contract
Gate decisions SHALL support:
- `approve`
- `reject`
- `conditional_approve`

Before decision, system SHALL present:
- assurance summary
- unresolved blockers
- risk conclusions
- verification outcome

Decision-to-status mapping SHALL be:
- `approve` => `approved`
- `reject` => `blocked`
- `conditional_approve` => `pending`

For `reject` and `conditional_approve`, at least one actionable reason SHALL be recorded.

#### Scenario: Conditional approve
- **WHEN** decision is `conditional_approve`
- **THEN** gate status SHALL remain pending until conditions are met

#### Scenario: Reject maps to blocked
- **WHEN** decision is `reject`
- **THEN** gate status SHALL be `blocked` and SHALL include explicit reasons

#### Scenario: Approve maps to approved
- **WHEN** decision is `approve`
- **THEN** gate status SHALL be `approved`

