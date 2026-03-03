## ADDED Requirements

### Requirement: Internal Five-Dimension Scoring Model
Routing SHALL use five internal dimensions in range `0..4`:
- `novelty`
- `ambiguity`
- `impact`
- `risk`
- `reversibility_cost`

Derived values:
- `discovery_score = novelty + ambiguity`
- `control_score = impact + risk + reversibility_cost`

Compatibility note:
- this MVP uses the common 5-dimension `N/A/I/R/V` model
- derived score formulas remain `discovery_score` and `control_score` for MVP consistency

#### Scenario: Valid derived values
- **WHEN** input dimensions are valid
- **THEN** derived values SHALL be computed deterministically

### Requirement: Score Input Source
Scores SHALL be produced by analyze logic at `S1_ANALYZE` (human-assisted or model-assisted).

`speclane new` SHALL NOT expose manual `--scores` CLI input in MVP.

#### Scenario: No scores flag
- **WHEN** operator runs `speclane new --help`
- **THEN** `--scores` SHALL NOT appear as a supported option

### Requirement: Canonical Score Persistence
Persisted state SHALL store only raw score fields with readable names:
- `novelty`, `ambiguity`, `impact`, `risk`, `reversibility_cost`

Derived values (`discovery_score`, `control_score`) SHALL be computed from raw fields and SHALL NOT be persisted as standalone state fields.

#### Scenario: Raw-only score persistence
- **WHEN** analyze completes
- **THEN** persisted state SHALL store raw score fields only, and derived values SHALL be recomputed when needed

### Requirement: Executable Route Final Grade
For executable intake classifications, routing SHALL output a single final governance grade only:
- `L1 | L2 | L3`

No secondary grade SHALL be treated as final outcome.
For `non_speclane` classifications, level fields SHALL be omitted.

#### Scenario: Single level output for executable routing
- **WHEN** routing classifies intake as executable
- **THEN** route result SHALL include exactly one final level grade

### Requirement: Non-Executable Intake Rejection
Before level routing, `S1_ANALYZE` SHALL classify whether the request is executable within speclane scope.

If intent is pure Q&A/advisory, or semantic analysis returns clarification-required (non-executable in current pass):
- routing SHALL return `non_speclane` classification (no level grade)
- `speclane new` SHALL reject request creation with remediation to use normal chat flow
- no `request_id` or runtime state files SHALL be created

MVP AI-first semantic classification contract:
- classification MUST be language-agnostic and support any single-language or mixed-language intake
- classifier output SHALL follow structured schema `intake_assessment`:
  - `intent_type`: `executable_change|advisory|question|mixed|unclear`
  - `is_executable`: boolean
  - `confidence`: number in `[0,1]`
  - `change_targets[]`: normalized paths/components/objects to be changed
  - `intended_delta`: concise summary of the requested change
  - `acceptance_anchor`: explicit desired outcome / done condition
  - `blocking_unknowns[]`: missing constraints that block safe execution
  - `auxiliary_signals[]` (optional): non-authoritative hints (for example, path tokens or domain phrases)
- built-in language lexicons SHALL NOT be required for executable-intent classification

Deterministic consumer rule over `intake_assessment`:
1. classify as `non_speclane` when all are true:
   - `intent_type in {advisory, question}`
   - `is_executable=false`
   - `confidence >= 0.75`
2. classify as `executable` when all are true:
   - `is_executable=true`
   - `confidence >= 0.65`
   - (`change_targets[]` is non-empty OR `intended_delta` is non-empty)
3. otherwise (`mixed|unclear`, low confidence, or missing executable anchors):
   - classify as `non_speclane`
   - emit remediation to clarify executable target/delta and rerun `speclane new`
   - emit rationale marker `non_speclane:clarification_required`

`blocking_unknowns[]` handling:
- non-empty `blocking_unknowns[]` SHALL NOT force `non_speclane` by itself when executable criteria are met
- executable requests with non-empty `blocking_unknowns[]` SHALL carry rationale marker `execution_unknowns_present`
- in auto mode, executable requests with non-empty `blocking_unknowns[]` SHALL route to `L3` discovery path

#### Scenario: Pure Q&A rejected from speclane
- **WHEN** intake is a pure question with no actionable change intent
- **THEN** routing SHALL classify as `non_speclane` and block level assignment

#### Scenario: Executable request passes intake gate
- **WHEN** intake semantically describes a concrete code/config change and classifier confidence meets executable threshold (for example: "update auth middleware timeout in `internal/auth/mw.go`")
- **THEN** routing SHALL classify as executable and continue level selection

#### Scenario: Multilingual executable request passes intake gate
- **WHEN** intake uses any language or mixed languages and semantically expresses concrete executable intent (for example: "请 update `internal/auth/mw.go` 的 timeout 处理")
- **THEN** routing SHALL classify as executable and continue level selection

#### Scenario: Path + issue with explicit delta is executable
- **WHEN** intake includes path/issue context and explicit requested change outcome (for example: "`internal/auth/mw.go` timeout regression, adjust middleware timeout strategy")
- **THEN** routing SHALL classify as executable and continue level selection

#### Scenario: Low-confidence intent requires clarification
- **WHEN** intake intent is mixed/unclear or confidence is below threshold
- **THEN** routing SHALL classify as `non_speclane` with remediation asking for explicit target + intended delta

#### Scenario: Executable request with blocking unknowns stays in speclane
- **WHEN** intake is executable by semantic threshold but `blocking_unknowns[]` is non-empty
- **THEN** routing SHALL keep `classification=executable`, attach unknowns rationale, and avoid `non_speclane` rejection

### Requirement: Guardrail Detection and Risk Floor
Guardrail domains SHALL be detected from request/context and persisted as canonical `domain_slug` values:
- `auth_authz`
- `security_credentials`
- `privacy_pii`
- `financial_flows`
- `schema_data_migration`
- `irreversible_operations`
- `external_api_contracts`

Normalization contract (input -> persisted canonical):
- `auth/authz` -> `auth_authz`
- `security/credentials` -> `security_credentials`
- `privacy/PII` -> `privacy_pii`
- `financial flows` -> `financial_flows`
- `schema/data migration` -> `schema_data_migration`
- `irreversible operations` -> `irreversible_operations`
- `external API contracts` -> `external_api_contracts`

When guardrail domain is detected, effective risk for routing SHALL be at least 3.

#### Scenario: Guardrail floor applied
- **WHEN** guardrail is detected and raw risk is below 3
- **THEN** effective risk SHALL be raised to 3 for routing

#### Scenario: Guardrail domain normalization
- **WHEN** intake/context implies `security/credentials` domain
- **THEN** persisted `guardrail_domain` SHALL be canonical `security_credentials`

### Requirement: Level Selection Mode
`speclane new` SHALL resolve level mode before routing:
- `--level L1|L2|L3` => fixed level mode
- `--level auto` => auto mode
- omitted `--level` => use `.speclane/config.yaml` `defaults.level_mode` when valid (`auto|L1|L2|L3`)
- omitted `--level` in interactive mode => prompt `auto|L1|L2|L3` with config-derived default preselected
- omitted `--level` in non-interactive mode => apply config-derived default directly
- if config value is missing/invalid => fallback to `auto` with deterministic remediation hint

Mode outputs:
- fixed mode => `level_source=user_selected`
- auto mode => `level_source=auto`

Fixed-level safety contract:
- fixed mode keeps selected `level` as route output
- hard safety conflicts SHALL be emitted as `blocking_conflicts[]` for command-layer enforcement
- command layer SHALL reject `speclane new` persistence when `blocking_conflicts[]` is non-empty

#### Scenario: Fixed level selection
- **WHEN** `speclane new "fix login" --level L1` is run
- **THEN** route result SHALL be `level=L1` and `level_source=user_selected`

#### Scenario: Config default level mode is applied
- **WHEN** operator omits `--level` and config contains `defaults.level_mode=L2`
- **THEN** routing mode SHALL use fixed-level `L2` semantics unless operator explicitly overrides via CLI flag

### Requirement: `new_project` and `major_refactor` Signal Derivation
Routing inputs for `new_project` and `major_refactor` SHALL be derived at `S1_ANALYZE` from semantic assessment + workspace context.

`new_project=true` when all conditions hold:
1. semantic assessment indicates greenfield/build-from-empty intent
2. confidence for this signal is `>= 0.65`
3. intake includes a concrete build target (repo/service/module/app/workspace), or workspace context confirms no in-scope source files yet

`major_refactor=true` when all conditions hold:
1. semantic assessment indicates architecture-scale refactor/migration intent
2. confidence for this signal is `>= 0.65`
3. scope touches at least two components/files/subsystems, or explicit whole-system scope is confirmed

If either signal is uncertain (confidence below threshold), it SHALL default to `false` and append rationale marker `signal_uncertain:<name>`.

Auxiliary keyword/phrase/path extraction MAY be used only as non-authoritative context; it SHALL NOT independently force `new_project` or `major_refactor` to `true`.

Compatibility note:
- these high-discovery triggers represent the highest discovery-control class from upstream governance models; MVP routes them to `L3` and records rationale markers in `routing_rationale[]`

#### Scenario: Major refactor signal from intake
- **WHEN** intake requests "re-architect auth and session modules across middleware and API boundaries"
- **THEN** `major_refactor` SHALL be `true` and auto-routing SHALL evaluate corresponding L3 rule

### Requirement: Auto-Level Algorithm
Auto mode SHALL compute level in this order:
1. classify executable intent; if non-executable, return `non_speclane`
2. apply guardrail risk floor
3. executable `blocking_unknowns[]` non-empty => `L3`
4. `new_project` or `major_refactor` => `L3`
5. `ambiguity >= 3` and `control_score >= 8` => `L3`
6. guardrail exists => `L3`
7. `control_score >= 8` => `L2`
8. otherwise => `L1`

Tie rule:
- If borderline across adjacent levels, choose higher governance level.

#### Scenario: Default deterministic route
- **WHEN** no L2/L3 conditions match
- **THEN** auto mode SHALL produce `L1`

#### Scenario: Guardrail request routes to L3
- **WHEN** guardrail domain is detected in auto mode
- **THEN** route result SHALL be `L3`

### Requirement: Route Result Contract
Route result SHALL include:
- `classification` (`executable|non_speclane`)
- `intake_assessment` (at minimum: `intent_type`, `is_executable`, `confidence`, `change_targets[]`, `intended_delta`, `acceptance_anchor`, `blocking_unknowns[]`)
- `scores` (raw)
- `guardrail_domain` (canonical `domain_slug`)
- `routing_rationale[]`
- `blocking_conflicts[]` (optional; non-empty only when fixed-level safety conflicts are present)

Optional intake-assessment hints:
- `auxiliary_signals[]` MAY be included as non-authoritative diagnostics/audit hints

Conditional fields:
- when `classification=executable`, route result SHALL include `level` and `level_source`
- when `classification=non_speclane`, route result SHALL omit `level` and `level_source`

Route result SHALL not require kind fields.

When route output is persisted into state files, the system SHALL write `route_snapshot` containing:
- `scores` (raw only)
- `guardrail_domain` (canonical `domain_slug`)
- `routing_rationale[]`
- `blocking_conflicts[]` (optional; present only when fixed-level safety conflicts exist)

Admission persistence SHALL also include full `intake_assessment` payload from `S1_ANALYZE` for classification audit/replay.

Derived values (`discovery_score`, `control_score`) SHALL be recomputed from raw scores and SHALL NOT be persisted as standalone fields.

Derived contract metadata (`required_artifacts`, `required_gates`, `required_checks`) SHALL be computed at read time from routed `level` rules and SHALL NOT be persisted in `route_snapshot` in MVP.

#### Scenario: Guardrail route result
- **WHEN** guardrail domain is detected
- **THEN** route result SHALL include guardrail domain and rationale entries

#### Scenario: `non_speclane` route omits level fields
- **WHEN** routing classifies intake as `non_speclane`
- **THEN** route result SHALL include `classification=non_speclane` and SHALL NOT include `level` or `level_source`

#### Scenario: Required contract metadata is derived, not persisted
- **WHEN** status/context needs required artifacts/gates/checks for current request
- **THEN** values SHALL be derived from routed level rules instead of loading persisted `route_snapshot` contract lists

### Requirement: State Persistence Target
Route output SHALL first persist to admission state.

- L1: admission state remains execution source
- L2/L3: admission and governed states share `request_id`; governed state becomes execution source

#### Scenario: Governed handoff record
- **WHEN** route selects L2/L3
- **THEN** governed change state SHALL persist `request_id`, and both lane records SHALL retain matching `route_snapshot`

### Requirement: Pivot and Rescore
Pivot recommendation SHALL be raised when any trigger occurs:
1. scope delta > 30%
2. two or more core assumptions invalidated
3. data model or external contract changed
4. two consecutive review failures from intent drift

Pivot entry states SHALL be `S6_RUN_WAVES`, `S7_REVIEW`, or `S8_VERIFY`.

Execution contract:
- triggers above produce deterministic `pivot_required` guidance
- actual pivot processing still requires explicit operator command `speclane pivot`
- after explicit pivot invocation, workflow SHALL transition to `S1_ANALYZE` and refresh analyze evidence first
- `G_pivot` SHALL be evaluated using refreshed analyze evidence and explicit pivot intent (`reroute` or `rescope`)
- `rescope` intent is valid only when pivot is requested from governed `S6_RUN_WAVES`; `S7/S8` `rescope` requests are precondition-rejected
- reroute/rescope SHALL be applied only after `G_pivot` is approved
- approved unchanged-level `rescope` targets are compound via analyze:
  - unchanged `L2`: `S6 -> S1 -> S4`
  - unchanged `L3`: `S6 -> S1 -> S3` (scope revalidation without discovery replay)

After pivot:
- recompute or reselect level
- append `level_history`
- update `last_level_update_at`
- update current lane state file(s)

#### Scenario: Direct lane escalation
- **WHEN** pivot changes level from L1 to L2
- **THEN** governed change SHALL be created and level history SHALL record the transition

#### Scenario: Pivot from verify forces analyze-first reroute
- **WHEN** pivot is approved while state is `S8_VERIFY`
- **THEN** workflow SHALL first transition to `S1_ANALYZE`, refresh analyze evidence, evaluate `G_pivot`, and only then persist reroute outputs
