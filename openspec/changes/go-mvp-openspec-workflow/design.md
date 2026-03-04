## Context

SpecLane is a greenfield Go implementation of a governance-first AI workflow CLI.

This MVP change is the **planning contract** for workflow/state/artifact behavior. The primary goal is internal consistency for L1/L2/L3 control flow and artifact governance, with explicit source attribution to OpenSpec, GSD, and Superpowers.

## Goals / Non-Goals

### Goals

- Keep one canonical state taxonomy: `S0..S8` + `DONE`
- Route first, then decide if governed artifacts are needed
- Enforce `L2/L3` shared governed artifact aircraft from `S4` onward
- Keep `L1` lightweight (no default `aircraft/changes/<slug>/` creation)
- Keep pure Q&A/advisory requests outside speclane lifecycle
- Remove CLI `--scores` but keep internal scoring model
- Persist level metadata safely without DB
- Keep gate model minimal and explicit (`G_scope/G_plan/G_pivot/G_ship`)
- Encode explicit source mapping for workflow semantics
- Provide deterministic CLI failure contract for script integration

### Non-Goals

- Runtime DB
- Multi-user/distributed locking
- Additional gate families
- Kind-based routing in MVP
- Silent auto-promotion/demotion across levels
- Making tool-adapter generation a hard dependency for core workflow execution

## Decisions

### DEC-01: Workflow Architecture = Admission + Execution

**Decision:** Split control flow into two phases while keeping one state taxonomy.

- Admission phase (all levels):
  - `S0_INTAKE -> S1_ANALYZE`
- Execution phase:
  - `L1`: direct lane `S6 -> S7 -> S8 -> DONE`
  - `L2`: governed lane `S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
  - `L3`: governed lane `S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`

This keeps the user-required canonical chain while removing contradictions around change creation.
Transition to `DONE` is command-gated by `spln done` for all lanes; state checks in `S7/S8` may auto-run, but do not implicitly finalize lifecycle status.

L1 lightweight trigger semantics:
- `spln do` is the execution owner for L1 lightweight progression
- after `S6` execution work completes, the same `spln do` invocation SHALL evaluate `S7` then `S8` in order
- if both lightweight checks pass, runtime state SHALL persist `current_state=S8_VERIFY` with done-ready projection (not `DONE`)
- if either lightweight check fails, runtime state SHALL transition back to `S6_RUN_WAVES` with deterministic blockers/remediation
- no separate L1-only review/verify subcommand is required in MVP

Admission responsibility split:
- `S0_INTAKE`: capture request metadata and level mode selection input
- `S1_ANALYZE`: compute route result in auto mode, or validate safety/readiness in fixed-level mode

`S0_INTAKE` does not compute final level.

SpecLane scope boundary:
- if intake is pure Q&A/advisory (non-executable), `spln new` SHALL reject request creation with routing label `non_spln`
- rejected advisory runs SHALL not create `request_id` or runtime state files
- executable-intent detection SHALL be AI-first and language-agnostic via structured semantic assessment (`intent_type`, `is_executable`, `confidence`, `change_targets[]`, `intended_delta`, `acceptance_anchor`, `blocking_unknowns[]`) with deterministic threshold consumption; keyword/path/domain matches are auxiliary hints only

### DEC-02: State Ownership Boundary

**Decision:** State persistence is split by control boundary.

- Admission and direct-lane state:
  - `.spln/runtime/admissions/<request_id>.yaml`
- Governed runtime change state (L2/L3 only):
  - `.spln/runtime/changes/<request_id>.yaml`
- Governed spec artifact bundle (Git-managed):
  - `aircraft/changes/<slug>/`

`L1` completes using admission state only (no governed artifact directory by default).

After governed handoff (`L2/L3`), admission state becomes sealed snapshot and governed runtime change state is the only mutable execution source.
Handoff key contract is single-key:
- admission and governed change share the same immutable `request_id` (canonical primary key)
- `action_history` is lane-local and SHALL NOT be copied across files at handoff
- `task_runs` is lane-local and SHALL NOT be copied across files at handoff

Retention strategy:
- runtime layout semantics are specification-owned and CLI-visible; `spln init` SHALL NOT scaffold `.spln/README.md` or other non-executable narrative files under `.spln/`
- no automatic admission deletion
- when governed archive completes:
  - spec artifact bundle moves to `aircraft/changes/archived/<slug>/`
  - runtime change state moves to `.spln/archive/changes/<request_id>.yaml`
  - linked `sealed_handoff` admission snapshot moves to `.spln/archive/admissions/<request_id>.yaml`
- when direct-lane archive completes:
  - admission runtime state moves to `.spln/archive/admissions/<request_id>.yaml`

### DEC-03: Change Creation Policy

**Decision:** Governed change artifacts are created only for `L2/L3`.

Creation boundary:
- Route finalization ends at `S1_ANALYZE`
- If level is `L2/L3`, create governed bundle `aircraft/changes/<slug>/` and runtime state `.spln/runtime/changes/<request_id>.yaml` before first governed state
- If level is `L1`, do not create governed bundle/runtime change state by default
- `spln new` persists first execution landing state by routed level:
  - `L1 -> S6_RUN_WAVES`
  - `L2 -> S4_SPEC_BUNDLE`
  - `L3 -> S2_DISCOVER`

Escalation boundary:
- If `L1` pivots/escalates to `L2/L3`, create governed change then and append `level_history`.

### DEC-04: Routing Contract (Single Final Grade)

**Decision:** Keep five internal score dimensions. Executable routing outputs a single final governance grade; non-executable intake returns `non_spln`.

- Internal raw dimensions: `novelty`, `ambiguity`, `impact`, `risk`, `reversibility_cost`
- Derived dimensions: `discovery_score`, `control_score`
- Executable final output: `L1 | L2 | L3`
- Non-executable output: `non_spln` (no level)

Compatibility note:
- this matches the baseline 5-dimension `N/A/I/R/V` model
- derived score formulas remain `discovery_score` and `control_score`

CLI behavior:
- Keep `--level auto|L1|L2|L3`
- Remove `--scores`
- In auto mode, `S1_ANALYZE` computes scores internally
- When `--level` is omitted, consume `.spln/config.yaml` `defaults.level_mode` as default mode/value
- If `defaults.level_mode` is missing or invalid, fallback to `auto` with deterministic remediation
- `new_project` / `major_refactor` route inputs are derived deterministically from intake + workspace signals at `S1_ANALYZE`
- auto-route floor is guardrail-safe:
  - executable intake with non-empty `blocking_unknowns[]` routes to `L3` discovery path (unknowns are not auto-rejected as `non_spln`)
  - guardrail-domain request => `L3`
  - non-guardrail high-control (`control_score >= 8`) => `L2`
- upstream highest-discovery triggers (`new_project` / `major_refactor` / high-ambiguity control pressure) are represented in MVP as `L3` with explicit rationale markers
- Persist only raw five-dimension scores; derived values are recomputed from raw fields

### DEC-05: Route Snapshot Durability

**Decision:** Route outputs required by pivots/gates SHALL be durably persisted as a route snapshot.

- persist `route_snapshot` in lane state records
- include canonical `guardrail_domain` (`domain_slug`), raw scores, `routing_rationale[]`, and optional `blocking_conflicts[]`
- keep raw scores only; derived scores remain read-time projections
- compute required contracts (`required_artifacts/gates/skills`) from level + guardrail rules at read time instead of persisting them in snapshot
- pivot/gate checks consume the latest persisted snapshot, not transient CLI-only output
- persist `intake_assessment` in admission state for classification auditability and deterministic replay/debug

Canonical `guardrail_domain` values:
- `auth_authz`
- `security_credentials`
- `privacy_pii`
- `financial_flows`
- `schema_data_migration`
- `irreversible_operations`
- `external_api_contracts`

### DEC-06: Request Key and Slug Contract

**Decision:** Admission and governed change records SHALL follow one fixed auditable keying scheme in MVP.

`request_v1` rules:
- `request_id` = UUIDv7 lowercase canonical string
- governed `slug` = `{title_kebab}`
- slug collisions append numeric suffixes (`-2`, `-3`, ...)
- both `request_id` and `slug` are immutable after creation

`request_id` is the only cross-lane primary key.
`slug` is a governed artifact label/path key (L2/L3 only), not a global identity key.

### DEC-07: Remove Redundant Schema-Version Fields

**Decision:** MVP SHALL NOT carry per-file `schema_version` fields in config/state/manifest contracts.

- `SpecLaneConfig`, `AdmissionState`, `ChangeState`, and `ChangeManifest` omit `schema_version`
- compatibility/migration versioning is out of MVP scope for this design package

### DEC-08: Active Request Resolution (MVP Single-Active)

**Decision:** Runtime request context resolution is implicit but deterministic in MVP.

- at most one active non-terminal request MAY exist across runtime lane files
- `spln new` is blocked while an active request exists
- request-scoped active-context commands (`do/done/cancel/pivot/analyze/review`) resolve exactly one active request from runtime state
- zero or multiple active requests are explicit blocking errors with remediation for request-scoped commands
- `spln context` and `spln status` remain diagnostics-first read surfaces and MAY run without unique active-request resolution
- mutating repair actions are explicit via `spln repair`
- safe auto-repair for multi-active ambiguity is narrow by design:
  - allowed when duplicate active records are a same-request handoff fault
  - different-request multi-active ambiguity is reported as non-repairable and requires explicit operator action

Future compatibility note:
- multi-active support can be introduced via explicit `--request <request_id>` targeting; this is intentionally out of MVP scope

Active definition:
- admission lane active: `admission_status=active`
- governed lane active: `change_status=active`
- `done`, `cancelled`, and `sealed_handoff` are non-active statuses

### DEC-09: Fixed-Level Safety Rule

**Decision:** `S1_ANALYZE` always runs, even in fixed-level mode.

- `level_source=user_selected` keeps selected level
- No silent level rewrite
- Hard safety conflicts block progression with explicit remediation
- for active-request `spln analyze`, hard conflicts persist blockers in `route_snapshot.blocking_conflicts[]` while in `S1_ANALYZE` and keep lane/level outcome unchanged until explicit `spln pivot`

### DEC-10: Governed Artifact Aircraft (L2/L3 Shared)

**Decision:** L2 and L3 share the same governed aircraft from `S4` onward.

Required governed artifacts:
- `change.yaml`
- `proposal.md`
- `spec.md`
- `design.md`
- `tasks.md`
- `assurance.md`

L3 additional required artifact:
- `explore.md`

MVP risk record strategy:
- no required standalone `risk.md`
- risk analysis is recorded in the risk section of `design.md`
- `assurance.md` references design risk decisions and records final execution/verification verdicts

`assurance.md` minimum structure (MVP):
- `Scope Summary`
- `Verification Verdict`
- `Evidence Index`
- `Residual Risks and Exceptions`
- `Archive Decision`
- structure checks only require canonical sections/heading presence; content-depth quality is evaluated by review layers (not by structure gate alone)

`assurance.md` retention strategy:
- keep as required governed artifact for closeout verdict and evidence index
- avoid duplicating full risk narrative that already exists in `design.md`

Artifact/runtime split:
- `aircraft/changes/<slug>/change.yaml` is the governed manifest artifact (Git-managed)
- `.spln/runtime/changes/<request_id>.yaml` is mutable runtime execution state
- governed per-artifact lifecycle/version authority lives in runtime `ChangeState.Artifacts` (manifest remains minimal metadata snapshot)
- manifest review boundary in MVP is structural (`R0`) only: `change.yaml` must satisfy presence/schema/identifier integrity, but does not require `R1/R2/R3` content-quality review layers

Naming rationale:
- `aircraft/` is retained as the governed artifact control plane namespace (bundle + archive lifecycle), distinct from mutable runtime state under `.spln/runtime/`

### DEC-11: Manifest Level Snapshot Semantics

**Decision:** `change.yaml` `created_at_level` is a governed-creation snapshot, not the live level authority.

- on governed creation, manifest `created_at_level` is initialized from admission route result
- after governed creation, live level authority is runtime change state (`level`, `level_source`, `level_history`, `last_level_update_at`)
- pivot-triggered level changes update runtime change state and history
- manifest snapshot level is intentionally stable after creation

### DEC-12: L1 Governance Skill Minimalism

**Decision:** L1 keeps no mandatory governance skill requirements by default.

- governance skill evidence is optional in L1
- L2/L3 retain level-enforced governance skill contracts

**Rationale:** preserve clear ceremony gap between direct lane (L1) and governed lane (L2/L3).

### DEC-13: Gate Applicability

**Decision:** Gates are governed-lane controls only.

- `G_scope`: L3 only (`S3 -> S4`)
- `G_plan`: L2/L3 (`S5 -> S6`)
- `G_pivot`: conditional
- `G_ship`: governed completion (`S8 -> DONE` for L2/L3)
- governed `S8` always runs `goal-verification` first, then runs `final-closeout` only when closeout evidence is missing/stale
- when guardrail domain exists, `G_ship` additionally requires guardrail high-risk checks to pass using deterministic check IDs/reasons
- guardrail `check_id` format is fixed as `<domain_slug>.<check_slug>` and MUST come from the catalog registry

`L1` does not require gate decisions by default.

### DEC-14: G_scope Worktree Evidence

**Decision:** `G_scope` requires dedicated worktree metadata to be recorded.

Required fields:
- `worktree_path`
- `worktree_branch`

This is validated together with discovery and scope-confirmation evidence.
For L3 MVP, `G_scope` also requires `explore.md` to exist and include minimum sections (`Objectives`, `Unknowns`, `Assumptions`, `Scope Boundaries`, `Validation Plan`) with non-empty content.
Worktree authenticity checks for `G_scope`:
- `worktree_path` exists and is accessible
- `worktree_path` is current-repository Git worktree
- checked-out branch at `worktree_path` equals persisted `worktree_branch`
Write point:
- `S3_SCOPE_CONFIRMATION` is the authoritative state that resolves and persists `worktree_path/worktree_branch` before `G_scope` evaluation.

### DEC-15: Level Durability and History

**Decision:** Level metadata must be durable and explicit.

Required fields:
- `level`
- `level_source`
- `level_history`
- `last_level_update_at`

`level_history` MUST always exist (`[]` allowed). No omission in YAML.
`last_level_update_at` is a denormalized cache for fast status/context reads and SHALL be updated with every level mutation.

Bounded-history contract:
- max entries are controlled by `execution.max_level_history_entries` (default `100`)
- when append exceeds cap, oldest entries are dropped deterministically

### DEC-16: Level Write Authority and Synchronization

**Decision:** Top-level level metadata is the only level write authority in runtime state.

- `route_snapshot` does not persist duplicated `level`/`level_source` fields in MVP
- level mutations update top-level fields only (`level`, `level_source`, `level_history`, `last_level_update_at`)
- this removes mirror-field drift risk and simplifies persistence invariants

### DEC-17: Persistence Model (No DB)

**Decision:** Use filesystem persistence with role-split serialization + lock + atomic write.

- YAML serialization (`yaml.v3`) for config/runtime state
- JSON serialization for evidence records
- lock file `.spln/state.lock`
- atomic write for all state/evidence index writes:
  1. write temp file in same directory
  2. fsync temp file
  3. rename temp file over target
  4. fsync parent directory
- stale temp artifacts from interrupted writes are reported by diagnostics and safely cleaned by `spln repair`
- read-only commands (`spln status`, `spln context`) SHALL read without acquiring mutation lock

No runtime DB in MVP.

### DEC-18: Runtime Status Projection

**Decision:** `next_ready_actions` and `blockers` are runtime projections, not persisted source-of-truth fields.

- compute from current state + lane + gate/artifact/task evidence
- emit in `spln status` / `spln context`
- do not persist as authoritative YAML state fields
- when active set is `0` or `>1`, `spln context` and `spln status` still return global diagnostics instead of failing request-scoped resolution
- mutating diagnostics repair is owned by explicit `spln repair` command, not `status`

### DEC-19: Mixed Serialization Rationale

**Decision:** Keep runtime/config state in YAML and governance evidence in JSON for MVP.

- YAML remains operator-friendly for editable state/config artifacts
- JSON keeps evidence records deterministic for machine ingestion and schema validation
- serializers are intentionally split by data role (human-edited state vs machine evidence)

### DEC-20: Interrupted Flow Repair-Forward Contract

**Decision:** Repair-forward covers interrupted governed creation and interrupted terminal archive migration.

New/create interruption class:
- fault signature: active admission exists, but governed runtime/artifacts are missing for same `request_id`
- `spln repair` recreates missing governed runtime/artifacts and seals admission handoff

Terminal/archive interruption class:
- fault signature: lane source record already terminal (`done` or `cancelled`) but request-scoped archive migration is incomplete
- `spln repair` SHALL complete archive migration idempotently using existing request-scoped archive rules

Status diagnostics report both classes with deterministic remediation without mutating state unless `spln repair` is explicitly invoked.

### DEC-21: Bounded Local Locking and Stale-Lock Repair

**Decision:** Local state locking is bounded and stale-lock handling is explicit diagnostics behavior.

- mutating commands that need state lock SHALL wait at most `execution.lock_wait_timeout_seconds`
- lock timeout SHALL fail command without mutating state
- runtime SHALL persist lock-holder metadata sidecar `.spln/state.lock.meta` for diagnostics:
  - `holder_pid`
  - `acquired_at`
  - `command`
- stale-lock cleanup is allowed only via `spln repair` when:
  - lock acquire timed out
  - metadata holder pid is not alive
  - lock age exceeds `execution.lock_stale_after_seconds`
- non-diagnostics mutating commands SHALL NOT force-unlock on timeout

### DEC-22: Storage Hygiene Controls

**Decision:** Runtime storage growth SHALL be bounded by explicit retention controls.

- evidence retention is controlled by `execution.evidence_retention_days` (default `30`)
- `spln repair` MAY perform deterministic evidence GC for expired, non-active evidence
- active-request evidence SHALL never be pruned by automatic retention
- low-disk auto-trigger is controlled by `execution.evidence_gc_low_disk_free_mb` (default `512`)
- when free disk space is below threshold, evidence-writing command flows SHALL run one opportunistic retention-GC pass before write retry

### DEC-23: Cancel Preemption for In-Flight Wave Tasks

**Decision:** `spln cancel` SHALL preempt currently running wave tasks deterministically.

- cancellation sets terminal lifecycle status first, then requests in-flight task interruption
- runtime SHALL send graceful interrupt signal first (`SIGINT`) to active task subprocesses
- graceful wait budget is controlled by `execution.cancel_grace_period_seconds` (default `10`)
- if task process is still alive after grace budget, runtime SHALL force terminate (`SIGKILL`) before archive migration
- interrupted tasks SHALL be recorded as cancelled/aborted outcomes in run evidence and action history

### DEC-24: Evidence Contract Extension

**Decision:** Governance evidence uses lean core required fields with conditional traceability fields.

Core required fields:
- `run_summary_version`
- `session_id`
- `skill_name`
- `version`
- `state`
- `verdict`
- `blockers[]`
- `references[]`
- `timestamp`

Optional traceability fields:
- `input_hash`
- `input_scope`
- `mitigation_target`
- `actor_id`
- `role`

`run_summary_version` value domain:
- pre-run-summary governance skills (`S1_ANALYZE`, `S3_SCOPE_CONFIRMATION`, `S5_PLAN_AUDIT`) MUST persist `run_summary_version=0`
- run-summary-bound skills (`S6_RUN_WAVES`, `S7_REVIEW`, `S8_VERIFY`) MUST persist `run_summary_version>=1`
- review/verify readiness checks MUST compare evidence version against the latest frozen run summary version

Conditional requiredness:
- run-summary-bound skills MUST persist non-empty `input_hash`
- pre-run-summary skills MAY omit `input_hash` in MVP

`session_id` contract:
- UUIDv7 lowercase canonical format
- one skill execution context binds to one `session_id`; concurrent subagent runs must use different values

`input_hash` contract:
- lowercase hex SHA-256 over canonical JSON payload
- canonical payload includes request/state/version + normalized scope/fingerprint identifiers consumed by the skill
- canonicalization uses sorted keys + UTF-8 + LF normalization

`mitigation_target` consistency contract:
- field is optional denormalized metadata for audit readability/indexing
- when present, value MUST match the registered skill-to-mitigation mapping for `skill_name`
- when absent, consumers derive mapping from `skill_name`
- mismatch invalidates governance readiness

Actor identity contract:
- `actor_id` is optional and, when present, stable actor identity for audit correlation across evidence/events
- `role` is optional and, when present, captures execution role (`implementer|reviewer|operator`)
- reviewer-independence checks use mandatory `session_id`; optional actor fields remain audit metadata

Evidence filename contract:
- deterministic readable pattern: `<session_id>--<skill_name>.json`
- collision handling appends `--<n>` suffix without overwrite

This contract keeps reviewer independence and stale-evidence detection while reducing mandatory payload width for MVP.

### DEC-25: Task-Run Identity Integrity

**Decision:** Task run map key and payload identity MUST be consistent.

- persisted key format remains `<task_id>__rv<run_summary_version>`
- parsed key values MUST equal payload fields (`TaskRun.TaskID`, `TaskRun.RunSummaryVersion`)
- mismatch SHALL fail persistence as a state-integrity error

### DEC-26: Core-MVP vs Adapter Sidecar Scope

**Decision:** Core workflow completion criteria exclude tool-adapter generation tracks.

- core MVP success is measured by routing/state/gate/wave/review/archive contracts
- tool-adapter generation remains optional sidecar capability
- adapter-specific checks run as optional verification track and SHALL NOT block core-MVP functional completion

### DEC-27: Wave Model (GSD-Aligned)

**Decision:** Keep GSD-style wave orchestration:

1. normalize tasks to DAG
2. topological layering
3. conflict split by target files
4. wave-sequential execution
5. retry/skip/abort/pivot loop
6. frozen run summary before review

Added MVP clarification:
- `task_kind` includes `other`
- `task_kind=other` is explicitly non-wave-governed (manual/non-standard path)
- run summaries use monotonic `run_summary_version` per request (starting at `1`)
- task traces and governance evidence carry `run_summary_version` for reviewer-independence comparison and stale-readiness invalidation
- frozen summaries are persisted as immutable run records at `.spln/evidence/runs/<request_id>/rv<run_summary_version>.json`
- lane states persist pointer `latest_frozen_run_summary_version` for deterministic review/verify reads

### DEC-28: Source Attribution (State-Level)

**Decision:** State semantics are source-mapped explicitly.

| State | Primary Source | Purpose |
|---|---|---|
| `S0_INTAKE` | SpecLane local | intake and route setup |
| `S1_ANALYZE` | SpecLane local + Superpowers | minimal analysis and uncertainty reduction |
| `S2_DISCOVER` | Superpowers | exploration-first behavior for L3 |
| `S3_SCOPE_CONFIRMATION` | Superpowers + SpecLane local | explicit scope confirmation and worktree readiness |
| `S4_SPEC_BUNDLE` | OpenSpec | governed artifact bundle alignment |
| `S5_PLAN_AUDIT` | SpecLane local | run-readiness audit and remediation loop |
| `S6_RUN_WAVES` | GSD | dependency-layered execution |
| `S7_REVIEW` | OpenSpec + SpecLane local | layered review loop |
| `S8_VERIFY` | OpenSpec + GSD | goal verification + conditional closeout refresh before ship |
| `DONE` | OpenSpec + SpecLane local | completion and default request-scoped archive |

Conflict precedence:
1. SpecLane local
2. OpenSpec/GSD/Superpowers reference

### DEC-29: Cancellation Lifecycle

**Decision:** Cancellation is a terminal lifecycle outcome, modeled via lane lifecycle status (not a new canonical state ID).

- canonical state taxonomy remains `S0..S8` + `DONE`
- cancellation is represented by lifecycle status fields:
  - admission/direct lane: `admission_status=cancelled`
  - governed lane: `change_status=cancelled`
- cancelled requests MUST have no next-ready actions and SHALL remain auditable
- `spln done` performs completion with default request-scoped archive
- `spln cancel` performs lifecycle termination and immediate request-scoped archive
- `spln cancel` sequence is deterministic: set terminal cancelled lifecycle status first, then archive request-scoped targets, then persist cancelled status in archived record
- MVP does not provide a separate done-without-archive or cancel-without-archive path

### DEC-30: CLI Failure Contract

**Decision:** CLI failures are machine-integrable by default using a stable exit-code taxonomy plus structured error envelope.

Source-of-truth boundary:
- this decision is the canonical definition for exit-code taxonomy and error envelope schema
- command-layer specs SHALL reference this decision and MUST NOT diverge from it

Exit-code taxonomy:
- `0`: success
- `2`: invalid usage / argument contract violation
- `3`: precondition blocked (for example no-active/ambiguous-active, lock timeout)
- `4`: state integrity/consistency failure
- `5`: governance blocked (gate/review/evidence blockers)
- `6`: runtime execution failure (unexpected internal/tool failure)

Failure output contract:
- command failure SHALL emit structured JSON error envelope on stderr
- envelope includes:
  - `error_code` (stable symbolic id)
  - `category`
  - `message`
  - `remediation`
  - `exit_code`
  - optional `request_id`
  - optional `details`
- human-readable text MAY be printed, but envelope SHALL remain parse-stable

`non_spln` rejection mapping:
- `spln new` rejection for non-executable intake uses:
  - `category=precondition_blocked`
  - `exit_code=3`
  - `error_code=non_spln_intent`

### DEC-31: Pivot Execution Ordering and Intent Contract

**Decision:** Pivot processing is analyze-first and intent-explicit before route mutation.

Pivot sequence:
1. transition to `S1_ANALYZE`
2. refresh analyze evidence for current request
3. evaluate `G_pivot` using refreshed analyze evidence plus explicit pivot intent kind
4. apply reroute/rescope only when `G_pivot` is approved

Pivot intent contract:
- CLI exposes `--kind reroute|rescope`
- default kind is `reroute`
- rescope path requires explicit `--kind rescope`
- rescope path is valid only from governed `S6_RUN_WAVES`; requests from `S7_REVIEW`/`S8_VERIFY` are precondition-blocked

## Data Models (MVP Contract)

### Runtime Config (`.spln/config.yaml`)

```go
type SpecLaneConfig struct {
    Tools         []string        `yaml:"tools"`
    Defaults      ConfigDefaults  `yaml:"defaults"`
    Execution     ConfigExecution `yaml:"execution"`
    Unknown       map[string]any  `yaml:",inline"` // preserve unknown top-level keys on rewrite
}

type ConfigDefaults struct {
    LevelMode string `yaml:"level_mode"` // auto|L1|L2|L3 (default auto)
}

type ConfigExecution struct {
    Parallelization       bool `yaml:"parallelization"`         // default true
    MaxRetriesPerTask     int  `yaml:"max_retries_per_task"`    // default 2
    LockWaitTimeoutSeconds int `yaml:"lock_wait_timeout_seconds"` // default 10
    LockStaleAfterSeconds  int `yaml:"lock_stale_after_seconds"`  // default 120
    CancelGracePeriodSeconds int `yaml:"cancel_grace_period_seconds"` // default 10
    EvidenceRetentionDays  int `yaml:"evidence_retention_days"`   // default 30
    EvidenceGCLowDiskFreeMB int `yaml:"evidence_gc_low_disk_free_mb"` // default 512
    MaxLevelHistoryEntries int `yaml:"max_level_history_entries"` // default 100
}
```

Config persistence rule:
- known keys are read/written via typed fields above
- unknown top-level keys SHALL be preserved on rewrite
- nested unknown keys inside known top-level sections (`defaults`, `execution`) are out of MVP compatibility scope and MAY be dropped on rewrite
- `execution.lock_wait_timeout_seconds` bounds lock acquire wait before failure
- `execution.lock_stale_after_seconds` defines stale-lock age threshold for diagnostics repair
- `execution.cancel_grace_period_seconds` bounds graceful cancel wait before forced termination
- `execution.evidence_retention_days` defines retention window for eligible evidence GC
- `execution.evidence_gc_low_disk_free_mb` defines low-disk threshold for opportunistic auto-GC trigger
- `execution.max_level_history_entries` bounds persisted `level_history` length (drop oldest on overflow)
- scheduled/explicit repair execution is owned by `spln repair`; low-disk opportunistic one-pass GC SHALL run inline in evidence-writing command flows when free disk is below threshold
- malformed/unparseable `.spln/config.yaml` is treated as state-integrity failure; `spln repair` SHALL back up broken config to `.spln/archive/config/config.yaml.broken.<timestamp>.yaml` and rewrite deterministic defaults

### Admission State (new)

```go
type AdmissionState struct {
    RequestID          string              `yaml:"request_id"` // UUIDv7, immutable primary key
    Title              string              `yaml:"title"`
    AdmissionStatus    AdmissionStatus     `yaml:"admission_status"` // active|done|cancelled|sealed_handoff
    IntakeAssessment   IntakeAssessment    `yaml:"intake_assessment"`
    Level              Level               `yaml:"level"`
    LevelSource        LevelSource         `yaml:"level_source"`
    LevelHistory       []LevelHistoryEvent `yaml:"level_history"`
    LastLevelUpdateAt  time.Time           `yaml:"last_level_update_at"` // denormalized cache for fast status/context reads
    RouteSnapshot      RouteSnapshot       `yaml:"route_snapshot"`
    CurrentState       string              `yaml:"current_state"`
    LatestFrozenRunSummaryVersion int      `yaml:"latest_frozen_run_summary_version"` // 0 when no frozen run summary exists
    TaskRuns           map[string]TaskRun  `yaml:"task_runs,omitempty"`      // key format: <task_id>__rv<run_summary_version>; L1 direct-lane traces; lane-local (no handoff copy)
    EvidenceRefs       map[string]string   `yaml:"evidence_refs"`             // non-task evidence index only; must not duplicate TaskRun.EvidenceRef
    ActionHistory      []ActionEvent       `yaml:"action_history,omitempty"`  // admission/direct timeline
    SealedAt           *time.Time          `yaml:"sealed_at,omitempty"`
    CreatedAt          time.Time           `yaml:"created_at"`
    UpdatedAt          time.Time           `yaml:"updated_at"`
}
```

Handoff snapshot semantic:
- when governed handoff is sealed, admission snapshot keeps `current_state=S1_ANALYZE` as immutable last admission-phase state

### Governed Runtime Change State (L2/L3)

```go
type ChangeState struct {
    RequestID          string                    `yaml:"request_id"` // same canonical request key as admission
    Slug               string                    `yaml:"slug"` // {title-kebab}[ -N ], immutable L2/L3 artifact label
    Title              string                    `yaml:"title"`
    ChangeStatus       ChangeStatus              `yaml:"change_status"` // active|done|cancelled
    Level              Level                     `yaml:"level"`
    LevelSource        LevelSource               `yaml:"level_source"`
    LevelHistory       []LevelHistoryEvent       `yaml:"level_history"`
    LastLevelUpdateAt  time.Time                 `yaml:"last_level_update_at"` // denormalized cache for fast status/context reads
    RouteSnapshot      RouteSnapshot             `yaml:"route_snapshot"`
    CurrentState       string                    `yaml:"current_state"`
    LatestFrozenRunSummaryVersion int            `yaml:"latest_frozen_run_summary_version"` // 0 when no frozen run summary exists
    WorktreePath       string                    `yaml:"worktree_path,omitempty"`
    WorktreeBranch     string                    `yaml:"worktree_branch,omitempty"`
    Artifacts          map[string]*ArtifactState `yaml:"artifacts"`
    Gates              map[string]GateStatus     `yaml:"gates"`
    TaskRuns           map[string]TaskRun        `yaml:"task_runs,omitempty"` // key format: <task_id>__rv<run_summary_version>; lane-local governed execution traces
    EvidenceRefs       map[string]string         `yaml:"evidence_refs"` // non-task evidence index only; must not duplicate TaskRun.EvidenceRef
    ActionHistory      []ActionEvent             `yaml:"action_history"`
    CreatedAt          time.Time                 `yaml:"created_at"`
    UpdatedAt          time.Time                 `yaml:"updated_at"`
}
```

### Governed Change Manifest Artifact (`aircraft/changes/<slug>/change.yaml`)

```go
type ChangeManifest struct {
    Slug           string `yaml:"slug"`
    Title          string `yaml:"title"`
    RequestID      string `yaml:"request_id"`
    CreatedAtLevel Level  `yaml:"created_at_level"` // governed-creation snapshot; runtime change state is live authority
}
```

### Evidence Record

```go
type EvidenceRecord struct {
    SkillName    string    `json:"skill_name"`
    Version      string    `json:"version"`
    RunSummaryVersion int   `json:"run_summary_version"` // 0 for pre-run-summary skills; >=1 for run-summary-bound skills
    SessionID    string    `json:"session_id"` // UUIDv7 lowercase canonical format
    InputHash    string    `json:"input_hash,omitempty"` // required for run-summary-bound skills; canonical-input SHA-256 lowercase hex
    MitigationTarget string `json:"mitigation_target,omitempty"` // optional; when present must match skill mapping
    State        string    `json:"state"`
    InputScope   []string  `json:"input_scope,omitempty"`
    ActorID      string    `json:"actor_id,omitempty"`
    Role         string    `json:"role,omitempty"` // optional; if present: implementer|reviewer|operator
    Verdict      string    `json:"verdict"`
    Blockers     []string  `json:"blockers"`
    References   []string  `json:"references"`
    Timestamp    time.Time `json:"timestamp"`
}
```

### Frozen Run Summary Record

Persisted path:
- `.spln/evidence/runs/<request_id>/rv<run_summary_version>.json`

```go
type RunSummaryRecord struct {
    RequestID         string    `json:"request_id"`
    RunSummaryVersion int       `json:"run_summary_version"`
    CompletedTasks    []string  `json:"completed_tasks"`
    NonPassTasks      []string  `json:"non_pass_tasks"`
    CarriedDebt       []string  `json:"carried_debt"`
    EvidenceSet       []string  `json:"evidence_set"`
    OpenBlockers      []string  `json:"open_blockers"`
    FrozenAt          time.Time `json:"frozen_at"`
}
```

Contract:
- records are immutable once written
- lane-state pointer `latest_frozen_run_summary_version` references latest frozen summary for request

### Supporting Types (MVP Explicit Definitions)

```go
type Level string

const (
    LevelL1 Level = "L1"
    LevelL2 Level = "L2"
    LevelL3 Level = "L3"
)

type LevelSource string

const (
    LevelSourceAuto         LevelSource = "auto"
    LevelSourceUserSelected LevelSource = "user_selected"
)

type AdmissionStatus string

const (
    AdmissionStatusActive       AdmissionStatus = "active"
    AdmissionStatusDone         AdmissionStatus = "done"
    AdmissionStatusCancelled    AdmissionStatus = "cancelled"
    AdmissionStatusSealedHandoff AdmissionStatus = "sealed_handoff"
)

type ChangeStatus string

const (
    ChangeStatusActive    ChangeStatus = "active"
    ChangeStatusDone      ChangeStatus = "done"
    ChangeStatusCancelled ChangeStatus = "cancelled"
)

type LevelHistoryEvent struct {
    From      Level     `yaml:"from"`
    To        Level     `yaml:"to"`
    Reason    string    `yaml:"reason"`
    Timestamp time.Time `yaml:"timestamp"`
}

type IntakeAssessment struct {
    IntentType       string   `yaml:"intent_type"`                 // executable_change|advisory|question|mixed|unclear
    IsExecutable     bool     `yaml:"is_executable"`
    Confidence       float64  `yaml:"confidence"`                  // 0..1
    ChangeTargets    []string `yaml:"change_targets,omitempty"`
    IntendedDelta    string   `yaml:"intended_delta,omitempty"`
    AcceptanceAnchor string   `yaml:"acceptance_anchor,omitempty"`
    BlockingUnknowns []string `yaml:"blocking_unknowns,omitempty"`
}

type ScoreSet struct {
    Novelty           int `yaml:"novelty"`            // raw score
    Ambiguity         int `yaml:"ambiguity"`          // raw score
    Impact            int `yaml:"impact"`             // raw score
    Risk              int `yaml:"risk"`               // raw score
    ReversibilityCost int `yaml:"reversibility_cost"` // raw score
}

// Derived values (`discovery_score`, `control_score`) are runtime-computed
// and SHALL NOT be persisted in RouteSnapshot.

type RouteSnapshot struct {
    Scores           ScoreSet `yaml:"scores"` // raw-only score persistence
    GuardrailDomain  string   `yaml:"guardrail_domain,omitempty"` // canonical domain_slug enum
    RoutingRationale []string `yaml:"routing_rationale,omitempty"`
    BlockingConflicts []string `yaml:"blocking_conflicts,omitempty"` // fixed-level safety conflicts persisted for deterministic remediation
}

// required_artifacts/required_gates/required_skills are runtime-derived
// from routed level + guardrail rules and are not persisted in MVP.

type ArtifactState struct {
    Version   int       `yaml:"version"`
    State     string    `yaml:"state"` // draft|in_review|approved|stale|frozen
    UpdatedAt time.Time `yaml:"updated_at"`
}

type GateStatus struct {
    State   string   `yaml:"state"` // pending|approved|blocked
    Reasons []string `yaml:"reasons,omitempty"`
}

type GateDecision string

const (
    GateDecisionApprove            GateDecision = "approve"
    GateDecisionReject             GateDecision = "reject"
    GateDecisionConditionalApprove GateDecision = "conditional_approve"
)

type TaskRun struct {
    TaskID      string   `yaml:"task_id"`
    RunSummaryVersion int `yaml:"run_summary_version"` // monotonic per request; binds task trace to frozen run summary
    ChangedFiles []string `yaml:"changed_files"`
    TestSummary string   `yaml:"test_summary,omitempty"`
    EvidenceRef string   `yaml:"evidence_ref,omitempty"` // deterministic task evidence path: .spln/evidence/tasks/<request_id>/rv<run_summary_version>/<task_id>.json (collision suffix: <task_id>--<n>.json)
    CommitRef   string   `yaml:"commit_ref,omitempty"`
    Verdict     string   `yaml:"verdict"` // pass|fail|blocked|timeout|incomplete
    Timestamp   time.Time `yaml:"timestamp"`
}

type ActionEvent struct {
    Action    string    `yaml:"action"`
    FromState string    `yaml:"from_state"`
    ToState   string    `yaml:"to_state"`
    ActorID   string    `yaml:"actor_id"`
    Timestamp time.Time `yaml:"timestamp"`
    Notes     string    `yaml:"notes,omitempty"`
}
```

Task run retention rule:
- `task_runs` map keys are `<task_id>__rv<run_summary_version>` to preserve historical traces across retries
- persistence validation enforces key/payload identity (`<task_id>` == `TaskRun.TaskID`, `<run_summary_version>` == `TaskRun.RunSummaryVersion`)
- latest-per-task views are derived at read time, not by overwriting historical entries
- `TaskRun.RunSummaryVersion` intentionally duplicates key-encoded version for payload self-containment and integrity checks

### Runtime Status View (Projection)

```go
type StatusView struct {
    LaneMode          string   `json:"lane_mode"` // admission_only|governed|diagnostics
    LifecycleStatus   string   `json:"lifecycle_status"` // admission_status or change_status by lane
    Level             Level    `json:"level"`
    LevelSource       LevelSource `json:"level_source"`
    CurrentState      string   `json:"current_state"`
    NextReadyActions  []string `json:"next_ready_actions"`
    Blockers          []string `json:"blockers"`
    EvidenceFreshness string   `json:"evidence_freshness"` // fresh|stale|unknown
}
```

### CLI Error Envelope (Failure Output)

```go
type CLIError struct {
    ErrorCode   string         `json:"error_code"`
    Category    string         `json:"category"`
    Message     string         `json:"message"`
    Remediation string         `json:"remediation"`
    ExitCode    int            `json:"exit_code"`
    RequestID   string         `json:"request_id,omitempty"`
    Details     map[string]any `json:"details,omitempty"`
}
```

## Command Surface Alignment

### Daily Path

- `spln init`
- `spln new`
- `spln do`
- `spln status`
- `spln context`
- `spln done`
- `spln cancel`

### Situational

- `spln pivot`
- `spln repair`

### Expert Override

- `spln analyze`
- `spln review`

Important behavior change:
- `spln new` rejects pure Q&A/advisory requests (use normal chat path instead).
- `spln new` does not always create governed change artifacts.
- `spln status` is a read-only unified state + integrity surface.
- `spln repair` is the explicit mutating diagnostics-repair command.
- command failures use stable non-zero exit codes and structured JSON error envelope.

## Risks / Trade-offs

### Risk: Dual state files add conceptual overhead

Mitigation: strict boundary rule (admission for route/direct, change for governed), single canonical `request_id` across lane states, and persisted `route_snapshot` for audit continuity.

### Risk: L1 direct lane loses artifact trace compared to governed lane

Mitigation: keep full admission record + evidence trail; support explicit pivot-based escalation.

### Risk: Upstream reference divergence

Mitigation: state-level source mapping with SpecLane-local precedence.

### Risk: Evidence freshness drift

Mitigation: enforce `session_id` plus conditional `input_hash` (mandatory for run-summary-bound evidence) and stale-evidence checks in review/verify.
