# action-workflow Specification

## Purpose
TBD - created by archiving change go-mvp-openspec-workflow. Update Purpose after archive.
## Requirements
### Requirement: Canonical State Taxonomy
The workflow SHALL use one canonical state taxonomy:

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

Naming convention:
- In state IDs, `S` means **State/Stage**.

Cancellation note:
- cancellation is modeled as lifecycle status termination (`admission_status=cancelled` or `change_status=cancelled`)
- cancellation does not introduce a new canonical state ID

#### Scenario: Cancellation stays outside canonical state IDs
- **WHEN** an active request is cancelled
- **THEN** lifecycle status SHALL become `cancelled` without adding a new state ID beyond `S0..S8` and `DONE`

### Requirement: Two-Phase Workflow Boundary
State progression SHALL follow a two-phase model:

1. **Admission phase** (all levels): `S0 -> S1`
2. **Execution phase** (level dependent):
  - L1 enters direct lane (`S6..S8`)
  - L2/L3 enter governed lane (`S2..S8` subset by level)

#### Scenario: Admission always runs first
- **WHEN** any request is started via `spln new`
- **THEN** `S0_INTAKE` and `S1_ANALYZE` SHALL execute before downstream path selection

### Requirement: Post-`spln new` State Landing
For executable requests, `spln new` SHALL persist the first execution state (not `S1_ANALYZE`) as the active `current_state`.

Landing states:
- `L1`: admission lane `current_state=S6_RUN_WAVES`
- `L2`: governed lane `current_state=S4_SPEC_BUNDLE` (linked admission snapshot is sealed at `S1_ANALYZE`)
- `L3`: governed lane `current_state=S2_DISCOVER` (linked admission snapshot is sealed at `S1_ANALYZE`)

`spln do` SHALL execute from this persisted landing state.

#### Scenario: L3 new lands at discovery state
- **WHEN** `spln new` routes to `L3`
- **THEN** governed state SHALL start at `S2_DISCOVER`, and first `spln do` SHALL execute discovery behavior (not re-run `S1`)

### Requirement: `non_spln` Intake Rejection
If admission analysis determines the request is pure Q&A/advisory, or clarification-required non-executable in current pass, workflow SHALL not instantiate a speclane request.

Terminology contract:
- routing classification label for this outcome SHALL be `non_spln`
- `pure Q&A/advisory` and `clarification-required` are explanatory subtypes of `non_spln` in MVP

MVP classification rule references routing-engine semantic contract:
- `S1_ANALYZE` SHALL produce structured `intake_assessment` (`intent_type`, `is_executable`, `confidence`, `change_targets[]`, `intended_delta`, `acceptance_anchor`, `blocking_unknowns[]`)
- classification MUST be language-agnostic and accept any language or mixed-language intake
- deterministic consumer thresholds SHALL be applied over `intake_assessment` using routing-engine semantic constants and decision order
- auxiliary keyword/path/domain signals MAY be used as hints only and SHALL NOT be authoritative gate predicates

Rejection behavior:
- stop after intake/analyze classification
- do not create `request_id`
- do not persist admission/change runtime state
- return remediation to use normal chat flow

#### Scenario: Pure Q&A blocked before execution lane
- **WHEN** `S1_ANALYZE` classifies intake as non-executable
- **THEN** workflow SHALL terminate without entering any level path

#### Scenario: Executable intake enters level routing
- **WHEN** `S1_ANALYZE` semantic assessment confirms executable change intent with confidence meeting threshold
- **THEN** workflow SHALL continue to level selection path mapping

### Requirement: S0 Intake Responsibility
`S0_INTAKE` SHALL capture request metadata and level-selection inputs only.

`S0_INTAKE` SHALL include:
- request/intent capture
- optional user-selected level mode capture (`--level` or interactive selection)

`S0_INTAKE` SHALL NOT:
- compute routing scores
- compute final level
- apply guardrail risk floor routing logic

#### Scenario: S0 does not compute level
- **WHEN** workflow is at `S0_INTAKE`
- **THEN** final level SHALL remain unresolved until `S1_ANALYZE` executes

### Requirement: Level Path Mapping
Level paths SHALL be:

- `L1`: `S0 -> S1 -> S6 -> S7 -> S8 -> DONE`
- `L2`: `S0 -> S1 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
- `L3`: `S0 -> S1 -> S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

Path rules:
- `L2` and `L3` SHALL share the identical mainline from `S4` onward.
- `L3` SHALL require discovery and scope confirmation before `S4`.
- `L1` SHALL skip `S2/S3/S4/S5` by default.

#### Scenario: L2/L3 shared aircraft mainline
- **WHEN** level is L2 or L3 and state reaches `S4_SPEC_BUNDLE`
- **THEN** remaining path SHALL be `S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

### Requirement: L1 Lightweight `S7/S8` Semantics
L1 SHALL keep `S7_REVIEW` and `S8_VERIFY` to preserve canonical state taxonomy alignment with governed lanes.

For L1:
- `S7_REVIEW` SHALL run lightweight review checks from direct-lane run summary (no governance skill/gate requirement)
- `S8_VERIFY` SHALL run lightweight completion checks from direct-lane task verdicts/evidence refs (no governance skill/gate requirement)
- pass => auto-advance `S7 -> S8`; `S8` pass marks direct lane done-ready
- transition to `DONE` remains command-gated via `spln done` (same rule as governed lanes)
- fail => remediation loop via `S7 -> S6` or `S8 -> S6`

Lightweight check criteria SHALL be:
- `S7_REVIEW` pass requires:
  - at least one direct-lane task run exists, and
  - no task run has `verdict` in `fail|blocked|timeout`, and
  - latest frozen run summary is present and internally consistent, and
  - latest frozen run summary version is `>=1`
- `S8_VERIFY` pass requires:
  - all direct-lane task runs are `pass`, and
  - every task marked `pass` has a resolvable `evidence_ref`, and
  - no unresolved blockers remain in runtime projection

Lightweight check failure handling SHALL be deterministic:
- `S7_REVIEW` failure => `S7 -> S6`
- `S8_VERIFY` failure => `S8 -> S6`

L1 lightweight check trigger contract:
- trigger owner is `spln do` (no dedicated L1 review/verify subcommand)
- when L1 executes from `S6_RUN_WAVES`, the same `spln do` invocation SHALL evaluate `S7_REVIEW` and then `S8_VERIFY` in order
- if both checks pass, runtime SHALL persist `current_state=S8_VERIFY` with done-ready projection and remediation to run `spln done`
- if either check fails, runtime SHALL persist `current_state=S6_RUN_WAVES` with deterministic blockers

#### Scenario: L1 auto-advance through lightweight review/verify
- **WHEN** level is L1 and direct-lane lightweight checks pass
- **THEN** workflow SHALL auto-advance through `S7` and `S8` without governed review/verify skills, and SHALL require explicit `spln done` for `S8 -> DONE`

#### Scenario: L1 verify fails on missing evidence
- **WHEN** level is L1 and a `pass` task run lacks resolvable `evidence_ref`
- **THEN** `S8_VERIFY` SHALL fail and transition to `S6_RUN_WAVES`

#### Scenario: L1 auto-checks run inside one do invocation
- **WHEN** level is L1 and `spln do` runs from `S6_RUN_WAVES`
- **THEN** command flow SHALL execute `S6` work and then evaluate `S7`/`S8` lightweight checks in the same invocation before returning

### Requirement: Change Creation Boundary
Governed change creation SHALL occur only for L2/L3.

- L2/L3: create governed bundle `aircraft/changes/<slug>/` and runtime state `.spln/runtime/changes/<request_id>.yaml` before first governed state
- L1: SHALL NOT create governed bundle/runtime change state by default

#### Scenario: L1 no governed change scaffolding
- **WHEN** route result is `L1`
- **THEN** state SHALL remain in admission/direct lane storage and no governed change directory SHALL be required

#### Scenario: L2 creates governed change
- **WHEN** route result is `L2`
- **THEN** `aircraft/changes/<slug>/` and `.spln/runtime/changes/<request_id>.yaml` SHALL be created before entering `S4_SPEC_BUNDLE`

### Requirement: `S2_DISCOVER` Execution Semantics (L3)
When `current_state=S2_DISCOVER`, `spln do` SHALL execute discovery-first preparation for governed scope confirmation.

`S2_DISCOVER` execution SHALL:
- produce or refresh L3 `explore.md` discovery content
- capture discovery outputs for:
  - objectives
  - unknowns
  - assumptions
  - provisional scope boundaries
  - validation approach
- record unresolved discovery blockers explicitly in runtime projection when present

`S2_DISCOVER` pass criteria:
- `explore.md` exists in governed artifact bundle
- required discovery sections are present with non-empty content
- no blocking discovery gap remains unrecorded

Transition rule:
- pass => `S2 -> S3`
- fail => remain in `S2` with deterministic remediation blockers

#### Scenario: Discovery pass advances to scope confirmation
- **WHEN** `S2_DISCOVER` pass criteria are satisfied
- **THEN** workflow SHALL transition to `S3_SCOPE_CONFIRMATION`

### Requirement: `S3_SCOPE_CONFIRMATION` Execution Semantics (L3)
When `current_state=S3_SCOPE_CONFIRMATION`, `spln do` SHALL execute scope confirmation and persist dedicated worktree metadata required by `G_scope`.

`S3_SCOPE_CONFIRMATION` execution SHALL:
- validate scope-confirmation evidence for current discovery outputs
- resolve dedicated worktree metadata from active workspace context:
  - `worktree_path`
  - `worktree_branch`
- persist resolved metadata into governed runtime change state before `G_scope` evaluation

`S3_SCOPE_CONFIRMATION` pass criteria:
- scope-confirmation evidence is present and valid
- `worktree_path` and `worktree_branch` are both non-empty in governed runtime state
- `worktree_path` exists and is accessible
- `worktree_path` resolves to current repository Git worktree
- checked-out branch at `worktree_path` matches persisted `worktree_branch`

Transition rule:
- pass => evaluate `G_scope`; if approved, `S3 -> S4`
- fail => remain in `S3` with deterministic remediation blockers

#### Scenario: Missing worktree metadata keeps workflow in S3
- **WHEN** `S3_SCOPE_CONFIRMATION` cannot resolve `worktree_path` or `worktree_branch`
- **THEN** workflow SHALL remain in `S3_SCOPE_CONFIRMATION` and return remediation to provide/fix dedicated worktree context

#### Scenario: Invalid worktree binding keeps workflow in S3
- **WHEN** `S3_SCOPE_CONFIRMATION` finds non-existent worktree path or branch mismatch against `worktree_branch`
- **THEN** workflow SHALL remain in `S3_SCOPE_CONFIRMATION` with remediation to repair dedicated worktree binding before `G_scope`

### Requirement: `S4_SPEC_BUNDLE` Execution Semantics (L2/L3)
When `current_state=S4_SPEC_BUNDLE`, `spln do` SHALL reconcile governed artifact bundle readiness before plan audit.

`S4_SPEC_BUNDLE` execution SHALL:
- ensure required governed artifacts exist for routed level
  - `L2`: `change.yaml`, `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `assurance.md`
  - `L3`: L2 set + `explore.md`
- validate governed `tasks.md` canonical task-node structure (heading + YAML block + required node fields) before plan-audit entry
- ensure required artifact identifiers align with runtime keys (`request_id`, `slug`)
- clear stale-planning blockers required for entering `S5_PLAN_AUDIT`

`S4_SPEC_BUNDLE` transition rule:
- pass => `S4 -> S5`
- fail => remain in `S4` with missing/stale artifact blockers

#### Scenario: Missing required artifact keeps workflow in S4
- **WHEN** `S4_SPEC_BUNDLE` detects missing required governed artifact
- **THEN** transition to `S5_PLAN_AUDIT` SHALL be blocked and workflow SHALL remain in `S4`

### Requirement: Gate Binding to State Transitions
Gate bindings SHALL be:

- `G_scope`: `S3 -> S4` (L3 only)
- `G_plan`: `S5 -> S6` (L2/L3)
- `G_pivot`: pivot-controlled transitions
- `G_ship`: `S8 -> DONE` (L2/L3)

#### Scenario: Direct lane has no mandatory gates
- **WHEN** level is L1 and no pivot/escalation is requested
- **THEN** transition checks SHALL not require governance gate approvals

### Requirement: Governed `S8` Closeout Sub-Step
For governed lane (`L2/L3`), `S8_VERIFY` SHALL run ordered sub-steps before `G_ship` evaluation:
1. `goal-verification` (always required)
2. `final-closeout` refresh (required only when closeout evidence is missing or stale)

`final-closeout` SHALL NOT be modeled as a standalone workflow state.

#### Scenario: Governed S8 runs closeout refresh
- **WHEN** level is L2/L3 and `S8_VERIFY` detects stale/missing closeout evidence
- **THEN** `S8_VERIFY` SHALL run `final-closeout` before evaluating `G_ship`

### Requirement: Transition and Loop Rules
The workflow engine SHALL enforce the following main and loop transitions.

Main transitions:
- `S0 -> S1`
- `S1 -> S6` (L1)
- `S1 -> S4` (L2)
- `S1 -> S2` (L3)
- `S2 -> S3`
- `S3 -> S4`
- `S4 -> S5`
- `S5 -> S6`
- `S6 -> S7`
- `S7 -> S8`
- `S8 -> DONE`

Completion transition rule:
- `S8 -> DONE` SHALL be command-gated by explicit `spln done` in all lanes; auto-check pass alone does not finalize lifecycle status

Loop transitions:
- `S5 -> S4` when plan-audit fails or planning artifacts are stale
- `S6 -> S6` for retry
- `S6 -> S1` for pivot-triggered re-analysis and reroute
- `S6 -> S4` only for explicit L2 rescope after analyze, when effective level remains `L2`
- `S6 -> S3` only for explicit L3 rescope after analyze, when effective level remains `L3` (re-run scope confirmation and `G_scope`)
- `S7 -> S1` for pivot-triggered re-analysis and reroute
- `S7 -> S6` when review fails
- `S8 -> S7` for explicit review override re-entry
- `S8 -> S1` for pivot-triggered re-analysis and reroute
- `S8 -> S6` when verify fails

Override transition rule:
- from any active non-`DONE` state, explicit `analyze` override MAY transition to `S1_ANALYZE`
- terminal lifecycle statuses (`cancelled`) SHALL block further progression transitions

Governed rescope trigger rule:
- rescope SHALL require explicit operator intent via `spln pivot`
- rescope path is valid only when pivot is invoked from governed `S6_RUN_WAVES`
- rescope requests from `S7_REVIEW`/`S8_VERIFY` SHALL be rejected with remediation to re-enter `S6_RUN_WAVES`
- transition SHALL require `G_pivot` approval with `rescope` request kind
- `L2` rescope target is `S4_SPEC_BUNDLE`
- `L3` rescope target is `S3_SCOPE_CONFIRMATION` (scope/worktree revalidation and `G_scope` re-evaluation before re-entering `S4`)
- automatic implicit `S6 -> S4`/`S6 -> S3` transitions without explicit operator rescope signal are not allowed
- if analyze/reroute changes level, transition SHALL follow reroute path from `S1` (not forced `S6 -> S4` or `S6 -> S3`)

#### Scenario: Plan-audit remediation loop
- **WHEN** plan-audit at `S5` fails
- **THEN** workflow SHALL return to `S4_SPEC_BUNDLE`

#### Scenario: Pivot from review triggers explicit re-analysis
- **WHEN** pivot is approved while state is `S7_REVIEW`
- **THEN** workflow SHALL transition to `S1_ANALYZE` before selecting next lane/state path

#### Scenario: Verify-stage review override
- **WHEN** operator invokes review override while state is `S8_VERIFY`
- **THEN** workflow SHALL transition `S8_VERIFY -> S7_REVIEW` before executing review

#### Scenario: Explicit L2 rescope keeps level and returns to S4
- **WHEN** operator requests rescope from `S6`, analyze completes, and effective level remains `L2`
- **THEN** workflow SHALL transition to `S4_SPEC_BUNDLE` for governed artifact resynchronization

#### Scenario: Explicit L3 rescope keeps level and returns to S3
- **WHEN** operator requests rescope from `S6`, analyze completes, and effective level remains `L3`
- **THEN** workflow SHALL transition to `S3_SCOPE_CONFIRMATION` before `S4_SPEC_BUNDLE`

#### Scenario: Rescope request from review/verify is rejected
- **WHEN** operator requests `spln pivot --kind rescope` while current state is `S7_REVIEW` or `S8_VERIFY`
- **THEN** workflow SHALL reject request with remediation to resume execution and invoke rescope from governed `S6_RUN_WAVES`

### Requirement: State Storage Ownership
State ownership SHALL be explicit:

- Admission/direct-lane states: `.spln/runtime/admissions/<request_id>.yaml`
- Governed-lane runtime states: `.spln/runtime/changes/<request_id>.yaml`
- Governed-lane spec artifacts: `aircraft/changes/<slug>/`

#### Scenario: L3 state handoff
- **WHEN** L3 transitions from `S1` into governed flow
- **THEN** admission state SHALL become sealed snapshot, governed state SHALL record `request_id`, and governed state SHALL become execution source of truth

### Requirement: State Control Matrix
Runtime checks SHALL use this control matrix:

| State | Levels | State Store | Exit Gate | Skill Binding |
|---|---|---|---|---|
| `S0_INTAKE` | L1/L2/L3 | admission | none | none |
| `S1_ANALYZE` | L1/L2/L3 | admission | none | `intake-analysis` required in auto mode and fixed L2/L3; optional in fixed L1 |
| `S2_DISCOVER` | L3 | change | none | none |
| `S3_SCOPE_CONFIRMATION` | L3 | change | `G_scope` | `scope-confirmation` |
| `S4_SPEC_BUNDLE` | L2/L3 | change | none | none |
| `S5_PLAN_AUDIT` | L2/L3 | change | `G_plan` | `plan-audit` |
| `S6_RUN_WAVES` | L1/L2/L3 | admission (L1) / change (L2/L3) | none | none for L1; `wave-orchestration` for L2/L3 |
| `S7_REVIEW` | L1/L2/L3 | admission (L1) / change (L2/L3) | none | lightweight auto-check for L1; `artifact-review` for L2/L3 |
| `S8_VERIFY` | L1/L2/L3 | admission (L1) / change (L2/L3) | `G_ship` (L2/L3 only) | lightweight auto-check for L1; `goal-verification` + conditional `final-closeout` for L2/L3 |
| `DONE` | L1/L2/L3 | lane store | n/a | n/a |

#### Scenario: Matrix-driven state checks
- **WHEN** executor evaluates a transition
- **THEN** it SHALL use this matrix for state-store, gate, and skill requirements

Interpretation note:
- `intake-analysis` is the evidence contract bound to `S1_ANALYZE`; routing score computation remains owned by analyze logic in routing-engine
- L1 `S7/S8` lightweight outcomes are represented by task verdicts + runtime projection blockers; they SHALL NOT require or emit governed gate (`GateStatus`) records

#### Scenario: Auto mode always executes intake-analysis
- **WHEN** `spln new` runs in `--level auto`
- **THEN** `S1_ANALYZE` SHALL require and execute `intake-analysis` before final level is resolved

### Requirement: Fixed-Level Analyze Behavior
`S1_ANALYZE` SHALL run even when level is user-fixed.

`S1_ANALYZE` responsibility split:
- auto mode: compute scores and final level route result
- fixed mode: validate safety/readiness against selected level without silent rewrite

When `level_source=user_selected`:
- run lightweight safety and readiness checks
- keep selected level unchanged unless operator explicitly pivots/adjusts
- for `spln new`, hard conflicts SHALL fail-fast before request-state creation (no persisted active request)
- for active-request analyze override, hard conflicts SHALL keep lane/level routing outcome unchanged (no reroute/rescope), persist analyze blockers in `route_snapshot.blocking_conflicts[]` while `current_state=S1_ANALYZE`, and return actionable remediation

#### Scenario: Fixed L1 blocked by guardrail conflict
- **WHEN** level is fixed to L1 and analyze detects guardrail constraints requiring governed controls
- **THEN** `spln new` SHALL fail before creating runtime request state and return remediation to rerun with `--level auto|L3`

#### Scenario: Analyze override conflict blocks progression without reroute
- **WHEN** active request runs `spln analyze` and fixed-level safety detects hard conflict
- **THEN** workflow SHALL persist analyze blockers in `route_snapshot.blocking_conflicts[]` while `current_state=S1_ANALYZE`, and SHALL NOT apply lane/level reroute until explicit `spln pivot`
