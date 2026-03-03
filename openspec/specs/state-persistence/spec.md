# state-persistence Specification

## Purpose
TBD - created by archiving change go-mvp-openspec-workflow. Update Purpose after archive.
## Requirements
### Requirement: Filesystem Persistence (No DB)
MVP SHALL use filesystem persistence only (YAML for config/runtime state and JSON for evidence records). No runtime database SHALL be required.

Persistent files:
- `.spln/config.yaml`
- `.spln/runtime/admissions/<request_id>.yaml`
- `.spln/runtime/changes/<request_id>.yaml` (governed runtime state only)
- `aircraft/changes/<slug>/change.yaml` (governed manifest artifact)
- `.spln/evidence/skills/<request_id>/<evidence-file>.json` (governance evidence)
- `.spln/evidence/tasks/<request_id>/rv<run_summary_version>/<task_id>.json` (task evidence records)
- `.spln/evidence/runs/<request_id>/rv<run_summary_version>.json` (frozen run summary snapshots)
- `.spln/archive/admissions/<request_id>.yaml` (archived admission records: direct-lane done/cancel and governed sealed-handoff snapshots)
- `.spln/archive/changes/<request_id>.yaml` (archived governed runtime state)
- `.spln/archive/config/config.yaml.broken.<timestamp>.yaml` (backup of malformed config repaired by `spln repair`)

Narrative/operator guidance documents are out of runtime persistence scope and SHALL NOT be required for lifecycle correctness.

#### Scenario: No DB bootstrap
- **WHEN** `spln init` runs in a fresh repo
- **THEN** the system SHALL be fully functional without provisioning a DB

### Requirement: Typed Serialization
All state reads and writes SHALL use typed Go structs with YAML tags (`yaml.v3`).

#### Scenario: Round-trip fidelity
- **WHEN** admission or change state is saved then loaded
- **THEN** fields SHALL round-trip without semantic loss

### Requirement: Config Unknown-Key Preservation
`.spln/config.yaml` writes SHALL preserve unknown top-level keys on rewrite.

Implementation SHALL use typed known-key decoding with top-level unknown key passthrough.
Nested unknown keys inside known top-level sections (`defaults`, `execution`) are out of MVP compatibility scope and MAY be dropped on rewrite.

#### Scenario: Preserve unknown config keys
- **WHEN** config contains unknown top-level keys outside the MVP schema and a known key is updated
- **THEN** unknown keys SHALL remain present after rewrite

### Requirement: Config Parse/Validation Recovery
Config corruption SHALL be detected and repaired via explicit repair flow.

Recovery contract:
- malformed/unparseable `.spln/config.yaml` SHALL be treated as state-integrity failure for config-consuming commands
- `spln repair` SHALL back up broken config to `.spln/archive/config/config.yaml.broken.<timestamp>.yaml` and rewrite deterministic MVP defaults
- rewrite SHALL preserve unknown top-level keys only when source config can be parsed

#### Scenario: Malformed config is recoverable via repair
- **WHEN** `.spln/config.yaml` is malformed
- **THEN** `spln repair` SHALL be able to restore runnable config defaults with backup of broken source

### Requirement: Fixed ID Generation Contract (MVP)
MVP SHALL use a single deterministic ID generation scheme:
- `request_id=uuidv7`
- governed `slug={title_kebab}` (L2/L3 only)
- slug collisions use numeric suffixing (`-2`, `-3`, ...)

This scheme is fixed in MVP (not configurable).

#### Scenario: Admission id follows request_v1
- **WHEN** `spln new` creates an admission record
- **THEN** `request_id` SHALL be UUIDv7 lowercase canonical format

### Requirement: Runtime Projection Fields Are Not Authoritative State
`next_ready_actions` and `blockers` SHALL be treated as runtime projection outputs, not persisted source-of-truth state fields.

They SHALL be computed from current state, lane metadata, gate status, artifact freshness, and task/evidence results at read time.

#### Scenario: Projection recompute on status read
- **WHEN** `spln status` is run after gate/artifact changes
- **THEN** next-ready actions and blockers SHALL reflect recomputed runtime projection without requiring YAML field sync

### Requirement: Atomic Writes
All state writes SHALL be atomic.

Atomic sequence contract:
1. write temp file in the same directory as target
2. `fsync` temp file
3. rename temp file over target
4. `fsync` parent directory entry

#### Scenario: Crash during write
- **WHEN** process crashes mid-write
- **THEN** target file SHALL remain old-or-new content only, never partial content

#### Scenario: Interrupted temp artifacts are repairable
- **WHEN** stale temp artifacts remain after interrupted atomic writes
- **THEN** diagnostics SHALL report them and `spln repair` SHALL safely clean them

### Requirement: Concurrency Lock
State mutations SHALL use exclusive lock `.spln/state.lock` (`gofrs/flock`).

Lock configuration:
- `execution.lock_wait_timeout_seconds` (default `10`)
- `execution.lock_stale_after_seconds` (default `120`)
- `execution.cancel_grace_period_seconds` (default `10`)
- `execution.evidence_retention_days` (default `30`)
- `execution.evidence_gc_low_disk_free_mb` (default `512`)
- `execution.max_level_history_entries` (default `100`)

Lock-holder diagnostics metadata SHALL be maintained at `.spln/state.lock.meta` with:
- `holder_pid`
- `acquired_at`
- `command`

Mutation contract:
1. acquire lock
2. load latest state
3. apply mutation
4. write atomically
5. release lock

Read-path contract:
- read-only diagnostics commands (`spln status`, `spln context`) SHALL NOT require acquiring mutation lock

#### Scenario: Concurrent updates
- **WHEN** two workers attempt simultaneous state updates
- **THEN** updates SHALL be serialized and no write SHALL be lost

#### Scenario: Lock wait timeout blocks mutation
- **WHEN** lock cannot be acquired within `execution.lock_wait_timeout_seconds`
- **THEN** mutation command SHALL fail without state change and return deterministic lock-timeout remediation

Stale-lock remediation policy:
- non-diagnostics mutating commands SHALL NOT force-unlock
- `spln repair` MAY clean stale lock artifacts when all conditions hold:
  1. lock acquire timed out
  2. `state.lock.meta` holder pid is not alive
  3. lock age exceeds `execution.lock_stale_after_seconds`

#### Scenario: Repair clears stale lock metadata
- **WHEN** stale-lock conditions are satisfied and `spln repair` is invoked
- **THEN** repair SHALL clear stale lock artifacts and report repaired status before subsequent state diagnostics

### Requirement: Active Request Discovery and Uniqueness (MVP)
The persistence layer SHALL support deterministic active-request discovery from runtime lane files.

Active discovery set:
- admission runtime files with `admission_status=active`
- governed runtime files with `change_status=active`

Non-active statuses (`done`, `cancelled`, `sealed_handoff`) SHALL be excluded from the active set.

MVP uniqueness rule:
- runtime SHALL permit at most one active request across admissions + governed changes
- `spln new` SHALL reject creation if active set size is `>= 1`
- request-scoped active-context commands (`do`, `done`, `cancel`, `pivot`, `analyze`, `review`) SHALL block when active set size is `0` or `>1`
- `spln context` and `spln status` SHALL still be allowed in diagnostics mode when active set size is `0` or `>1`
- `spln repair` MAY run in diagnostics mode when active set size is `0` or `>1`

#### Scenario: Single active request is resolvable
- **WHEN** runtime contains exactly one active request across lane files
- **THEN** active-context command resolution SHALL bind to that `request_id`

#### Scenario: Multiple active requests are rejected
- **WHEN** runtime contains more than one active request
- **THEN** resolver SHALL return ambiguity error and block state mutation commands

### Requirement: Admission State Persistence
The system SHALL provide `LoadAdmission(request_id)` and `SaveAdmission(state)` for `.spln/runtime/admissions/<request_id>.yaml`.

Admission state SHALL be authoritative for:
- `S0/S1` for all in-scope speclane levels
- L1 direct lane execution (`S6/S7/S8`)
- route result and level metadata before governed handoff
- semantic intake classification output (`intake_assessment`) for auditability
- L1 direct-lane execution traces (`task_runs`, `action_history`)
- frozen run summary pointer (`latest_frozen_run_summary_version`) for direct-lane review/verify readiness
- non-task evidence index (`evidence_refs`)

#### Scenario: L1 direct lane persistence
- **WHEN** level is L1
- **THEN** state progression SHALL be persisted in admission YAML without requiring change YAML

#### Scenario: L1 task output durability
- **WHEN** L1 executes direct-lane tasks in `S6/S7/S8`
- **THEN** task outputs (`changed_files`, `verdict`, `evidence_ref`) SHALL be persisted via admission trace fields

#### Scenario: Admission id immutability
- **WHEN** admission record is persisted after creation
- **THEN** `request_id` SHALL remain immutable across subsequent writes

### Requirement: Task Run Identity and Historical Retention
`task_runs` SHALL retain historical execution traces without overwrite.

Map-key contract (admission + governed states):
- persisted key format: `<task_id>__rv<run_summary_version>`
- same `task_id` with different `run_summary_version` SHALL create distinct entries
- persistence SHALL reject writes that overwrite an existing key with different payload
- persistence SHALL validate key/value consistency:
  - parsed `<task_id>` from map key MUST equal `task_runs[*].task_id`
  - parsed `<run_summary_version>` from map key MUST equal `task_runs[*].run_summary_version`

Read model note:
- "latest per task" views MAY be projected at read time by selecting max `run_summary_version` for each `task_id`

#### Scenario: Retry preserves prior task trace
- **WHEN** task `T1` is rerun and produces `run_summary_version=2`
- **THEN** state SHALL keep both `T1__rv1` and `T1__rv2` entries

#### Scenario: Task-run key/value mismatch is rejected
- **WHEN** a `task_runs` entry key `T1__rv2` carries payload `task_id=T1` and `run_summary_version=1`
- **THEN** persistence validation SHALL reject the write with state-integrity remediation

### Requirement: Frozen Run Summary Persistence
Frozen run summaries SHALL be persisted as first-class immutable records.

Storage contract:
- summary file path: `.spln/evidence/runs/<request_id>/rv<run_summary_version>.json`
- file content SHALL include at least:
  - `request_id`
  - `run_summary_version`
  - `completed_tasks[]`
  - `non_pass_tasks[]`
  - `carried_debt[]`
  - `evidence_set[]`
  - `open_blockers[]`
  - `frozen_at`
- summary files SHALL be immutable once written

State-pointer contract:
- admission and governed lane states SHALL persist `latest_frozen_run_summary_version`
- pointer value SHALL be `0` when no frozen summary exists yet
- when a new frozen summary is emitted, pointer SHALL advance monotonically to that summary version

#### Scenario: Frozen summary pointer advances
- **WHEN** request emits a new frozen summary `rv3`
- **THEN** persisted `latest_frozen_run_summary_version` SHALL become `3` and summary file `.spln/evidence/runs/<request_id>/rv3.json` SHALL exist

### Requirement: `non_spln` Intake Has No Persistence Side Effects
If intake/analyze classifies a request as `non_spln` (pure Q&A/advisory or clarification-required non-executable), persistence layer SHALL not create admission/change records.

#### Scenario: `non_spln` classification writes nothing
- **WHEN** routing classification is `non_spln`
- **THEN** `.spln/runtime/admissions/` and `.spln/runtime/changes/` SHALL have no new files for that invocation

### Requirement: Evidence Ownership Boundary
Task-level evidence pointers SHALL be authoritative in `task_runs[*].evidence_ref`.

Task evidence reference path SHALL use deterministic request/run-scoped layout:
- `.spln/evidence/tasks/<request_id>/rv<run_summary_version>/<task_id>.json` (with suffix `--<n>` on collision)

`evidence_refs` SHALL be reserved for non-task evidence indexing and SHALL NOT duplicate task evidence pointers.
For both admission and governed change states, `evidence_refs` SHALL always be present as a map (`{}` allowed) and SHALL NOT be omitted.

#### Scenario: Duplicate evidence pointers are rejected
- **WHEN** a write attempts to store task evidence in both `task_runs[*].evidence_ref` and `evidence_refs`
- **THEN** persistence validation SHALL reject the duplicate and report remediation guidance

#### Scenario: Empty evidence index remains present
- **WHEN** state is written with no non-task evidence refs
- **THEN** persisted YAML SHALL include `evidence_refs: {}`

### Requirement: Governed Change State Persistence
The system SHALL provide `LoadChange(request_id)` and `SaveChange(state)` for `.spln/runtime/changes/<request_id>.yaml`.

Change state SHALL be authoritative for governed lane execution (`L2/L3`).
Change state SHALL include `change_status` lifecycle field with values:
- `active`
- `done`
- `cancelled`

Change state SHALL also include:
- `latest_frozen_run_summary_version` for governed review/verify readiness

#### Scenario: L3 governed persistence
- **WHEN** level is L3 after governed handoff
- **THEN** `S2..S8` progression SHALL be persisted in change YAML

#### Scenario: Governed cancellation persists lifecycle status
- **WHEN** governed request is cancelled
- **THEN** `change_status` SHALL be persisted as `cancelled` and treated as terminal

#### Scenario: Slug immutability
- **WHEN** governed change is persisted after creation
- **THEN** `slug` SHALL remain immutable across subsequent writes

### Requirement: Governed Worktree Metadata Persistence
Governed runtime state SHALL durably persist scope-confirmation worktree metadata required by `G_scope`:
- `worktree_path`
- `worktree_branch`

Write contract:
- metadata SHALL be written/updated during `S3_SCOPE_CONFIRMATION` execution before `G_scope` evaluation
- both fields SHALL be non-empty for passing `S3 -> S4` scope readiness
- persisted metadata SHALL pass authenticity checks before `G_scope` evaluation:
  - `worktree_path` exists and is accessible
  - `worktree_path` is a registered Git worktree of current repository
  - checked-out branch at `worktree_path` equals persisted `worktree_branch`

#### Scenario: Scope confirmation writes worktree metadata
- **WHEN** L3 request executes `S3_SCOPE_CONFIRMATION`
- **THEN** governed runtime state SHALL persist `worktree_path` and `worktree_branch` before `G_scope` decision

### Requirement: Governed Manifest Persistence
Governed change manifest SHALL be persisted at `aircraft/changes/<slug>/change.yaml`.

Manifest SHALL be Git-managed and SHALL include stable identifiers/contract metadata required for artifact review.
Per-artifact version/state authority SHALL remain in governed runtime `ChangeState.Artifacts`; manifest stays minimal and SHALL NOT carry per-artifact version map in MVP.

#### Scenario: Manifest exists for governed change
- **WHEN** route selects L2/L3
- **THEN** `aircraft/changes/<slug>/change.yaml` SHALL exist before governed readiness can pass

### Requirement: Manifest Level Snapshot Semantics
`aircraft/changes/<slug>/change.yaml` `created_at_level` SHALL be treated as governed-creation snapshot metadata.

Authoritative live level after governed creation SHALL be runtime change state:
- `level`
- `level_source`
- `level_history`
- `last_level_update_at`

Pivot/rescore updates SHALL mutate runtime change state level metadata, while manifest snapshot `created_at_level` remains stable.

#### Scenario: Pivot keeps manifest snapshot stable
- **WHEN** governed pivot changes runtime level from `L2` to `L3`
- **THEN** runtime change state level metadata SHALL update and manifest `change.yaml` `created_at_level` SHALL remain unchanged

### Requirement: Level Metadata Durability
Both admission state and governed change state SHALL include:
- `level`
- `level_source` (`auto|user_selected`)
- `level_history`
- `last_level_update_at`

`level_history` SHALL always exist (empty list allowed) and SHALL NOT be omitted.

Bounded-history contract:
- persisted `level_history` length SHALL NOT exceed `execution.max_level_history_entries`
- when appending exceeds max entries, oldest events SHALL be dropped deterministically
- `last_level_update_at` is a denormalized cache and SHALL update with each level mutation

#### Scenario: Level history always present
- **WHEN** a state file is created with no pivots yet
- **THEN** `level_history: []` SHALL be present

#### Scenario: Level history overflow is truncated
- **WHEN** appending a new level event would exceed `execution.max_level_history_entries`
- **THEN** persistence SHALL drop oldest history entries and keep most-recent events within configured cap

### Requirement: Route Snapshot Durability
Both admission and governed change states SHALL persist `route_snapshot` for gate/pivot auditability.

`route_snapshot` SHALL include:
- `scores` (raw only)
- `guardrail_domain` (canonical `domain_slug`)
- `routing_rationale[]`
- `blocking_conflicts[]` (optional; present only when fixed-level safety conflicts are detected)

`route_snapshot` SHALL NOT duplicate top-level level metadata.
Required contract metadata (`required_artifacts`, `required_gates`, `required_skills`) SHALL be derived from routed level + guardrail rules at read time and SHALL NOT be persisted in `route_snapshot` in MVP.

#### Scenario: Route snapshot survives handoff
- **WHEN** L2/L3 governed handoff occurs
- **THEN** admission and change records SHALL retain equivalent `route_snapshot` payloads

### Requirement: Intake Assessment Persistence
Admission state SHALL persist `intake_assessment` produced by `S1_ANALYZE` to preserve executable-classification evidence.

Minimum persisted fields:
- `intent_type`
- `is_executable`
- `confidence`
- `change_targets[]`
- `intended_delta`
- `acceptance_anchor`
- `blocking_unknowns[]`

#### Scenario: Analyze output includes intake assessment
- **WHEN** `S1_ANALYZE` completes for executable or non-executable decisioning
- **THEN** admission state SHALL persist structured `intake_assessment` payload for audit/replay

### Requirement: Top-Level Level Metadata Write Authority
Top-level level metadata fields (`level`, `level_source`, `level_history`, `last_level_update_at`) SHALL be the only persisted level authority in runtime lane state.

`route_snapshot` SHALL not carry duplicated level fields in MVP.

#### Scenario: Level metadata update writes top-level only
- **WHEN** pivot updates governed level metadata
- **THEN** persistence SHALL update top-level level fields without requiring mirrored level fields in `route_snapshot`

### Requirement: Governed Handoff Consistency
When entering governed lane (`L2/L3`), level metadata SHALL be copied from admission state to change state, preserving values and history.

After handoff:
- admission state SHALL be sealed as immutable routing snapshot (`sealed_at` recorded)
- sealed admission snapshot SHALL keep `current_state=S1_ANALYZE` as the last admission-phase executed state
- progression and level updates SHALL be written only to governed change state
- admission and governed change states SHALL share the same immutable `request_id`
- `action_history` SHALL remain lane-local (no cross-file copy at handoff); cross-lane timeline reconstruction SHALL use `request_id` across admission/change records
- `task_runs` SHALL remain lane-local (no cross-file copy at handoff); governed change `task_runs` starts empty at handoff and is populated by governed `S6` execution only

#### Scenario: Admission to change handoff
- **WHEN** route result requires L2 governed creation
- **THEN** runtime change YAML SHALL contain level metadata + `request_id` + `route_snapshot` matching admission YAML at handoff time, and admission SHALL be marked sealed with `current_state=S1_ANALYZE`

#### Scenario: Handoff does not duplicate action history
- **WHEN** governed handoff completes
- **THEN** pre-handoff admission `action_history` SHALL NOT be copied into governed change `action_history`; operators SHALL reconstruct full timeline by `request_id`

#### Scenario: Handoff does not duplicate task runs
- **WHEN** L1 admission state with existing `task_runs` is escalated to governed lane
- **THEN** pre-handoff admission `task_runs` SHALL NOT be copied into governed change `task_runs`

### Requirement: Analyze-Override Persistence Semantics
When `spln analyze` is invoked from active non-`S1` states, persistence SHALL keep prior execution traces for audit while resetting readiness projection.

Persistence contract:
- existing `task_runs` and `action_history` entries remain preserved (no destructive truncation)
- current state transitions to `S1_ANALYZE` for revalidation-only analyze execution
- analyze override SHALL refresh route metadata and projections but SHALL NOT mutate lane/level routing outcome (reroute is `spln pivot` only)
- prior frozen run-summary readiness is marked superseded and SHALL NOT be reused as current review/verify readiness
- fixed-level hard conflicts during analyze override SHALL persist blockers in `route_snapshot.blocking_conflicts[]` while `current_state=S1_ANALYZE`, and SHALL NOT mutate lane/level routing outcome

#### Scenario: Analyze override from S6 preserves traces
- **WHEN** governed request invokes `spln analyze` from `S6_RUN_WAVES`
- **THEN** historical task traces remain in state, but subsequent review/verify readiness SHALL be recomputed from post-analyze execution

### Requirement: Admission Record Lifecycle
Admission records SHALL have explicit lifecycle semantics:
- `active` while admission/direct lane is in progress
- `done` when L1 completes
- `cancelled` when admission/direct lane is explicitly cancelled
- `sealed_handoff` after governed handoff to L2/L3

Admission records are audit snapshots and SHALL NOT be auto-deleted by runtime execution.
Cancellation lifecycle records SHALL remain auditable after archive migration.

For governed archive:
- governed manifest/artifacts SHALL move from `aircraft/changes/<slug>/` to `aircraft/changes/archived/<slug>/`
- governed runtime change state SHALL move from `.spln/runtime/changes/<request_id>.yaml` to `.spln/archive/changes/<request_id>.yaml`
- linked `sealed_handoff` admission SHALL be migrated from runtime path to archive admissions path
- migrated admission SHALL remain immutable snapshot and retain original `request_id`

For direct-lane archive:
- runtime admission state SHALL move from `.spln/runtime/admissions/<request_id>.yaml` to `.spln/archive/admissions/<request_id>.yaml`
- archived admission SHALL retain immutable `request_id`

#### Scenario: Governed lane update after handoff
- **WHEN** L2/L3 progresses after handoff
- **THEN** admission state SHALL remain unchanged except read-only lookup fields

#### Scenario: Direct-lane cancellation lifecycle
- **WHEN** active admission/direct-lane request is cancelled
- **THEN** admission lifecycle SHALL transition `active -> cancelled`, runtime admission file SHALL migrate to `.spln/archive/admissions/<request_id>.yaml`, and cancelled status SHALL remain auditable in archive

#### Scenario: Governed cancellation archive lifecycle
- **WHEN** active governed request is cancelled
- **THEN** governed change lifecycle SHALL transition `active -> cancelled`, governed/runtime/artifact files SHALL migrate to request-scoped archive targets, and cancelled status SHALL remain auditable in archive

### Requirement: Governed-Create Partial Failure Repair (`spln new`)
For routed `L2/L3`, if `spln new` fails after admission write but before governed state/artifact creation, runtime SHALL treat this as a repairable consistency fault.

Repairable fault signature:
- admission exists with `admission_status=active`
- admission level is `L2` or `L3`
- no matching governed runtime state exists for the same `request_id`

`spln repair` SHALL repair forward by:
1. creating missing governed runtime state + governed artifact bundle from existing admission identifiers
2. setting governed landing `current_state` (`L2 -> S4_SPEC_BUNDLE`, `L3 -> S2_DISCOVER`)
3. sealing admission snapshot (`admission_status=sealed_handoff`, `current_state=S1_ANALYZE`)

Without explicit repair invocation, status SHALL report this fault as blocking diagnostics with deterministic remediation.

#### Scenario: Repair forward from orphaned governed admission
- **WHEN** runtime has an active L3 admission without matching governed change state due to interrupted `spln new`
- **THEN** `spln repair` SHALL recreate governed state/artifacts and complete handoff sealing instead of deleting the admission record

### Requirement: Interrupted Terminal Archive Migration Repair
For `spln done`/`spln cancel` archive flows, if terminal lifecycle is already persisted but request-scoped archive migration is incomplete, runtime SHALL treat it as a repair-forward consistency fault.

Repairable fault signature:
- source lane record is already terminal (`done` or `cancelled`)
- at least one required request-scoped archive target is missing or only partially migrated
- terminal lifecycle status is recoverable from existing runtime/archive records

`spln repair` SHALL repair forward by:
1. completing missing request-scoped archive moves idempotently
2. preserving terminal lifecycle status in archived record
3. ensuring no request is re-opened to active during repair

#### Scenario: Repair completes interrupted governed cancel archive
- **WHEN** governed `change_status=cancelled` is already persisted but one or more request-scoped archive targets are missing after interrupted cancellation flow
- **THEN** `spln repair` SHALL complete remaining archive migration idempotently and preserve `cancelled` status

#### Scenario: Repair completes interrupted direct done archive
- **WHEN** direct-lane `admission_status=done` is already persisted but runtime admission file still exists due to interrupted archive move
- **THEN** `spln repair` SHALL migrate remaining direct-lane archive target idempotently and preserve `done` status

### Requirement: Multi-Active Ambiguity Repair Boundary
`spln repair` SHALL handle multi-active ambiguity conservatively.

Safe auto-repair class:
- if admission and governed active records represent the same `request_id` handoff fault, repair MAY normalize to one active execution source by sealing admission (`sealed_handoff`) and keeping governed record active

Non-repairable class:
- multiple active records with different `request_id` values SHALL be reported as blocking diagnostics and SHALL NOT be auto-mutated

#### Scenario: Same-request dual-active is normalized
- **WHEN** runtime has active admission and active governed records for the same `request_id`
- **THEN** `spln repair` SHALL normalize to governed-active + sealed admission snapshot

#### Scenario: Different-request multi-active is not auto-fixed
- **WHEN** runtime has active records for multiple distinct `request_id` values
- **THEN** `spln repair` SHALL report non-repairable ambiguity without mutating ownership/lifecycle fields

### Requirement: Completion Archive Guard (`spln done` path)
Before `spln done` archive migration with governed change present, system SHALL validate governed runtime/manifest records:
- level metadata presence
- required gate/readiness status
- `change_status=active`

Before archive migration, system SHALL validate both locations:
- runtime state source: `.spln/runtime/changes/<request_id>.yaml`
- manifest/artifact source: `aircraft/changes/<slug>/change.yaml`

Missing required fields SHALL block archive with explicit remediation.

Governed `spln done` archive ordering SHALL be:
1. validate governed runtime preconditions while runtime `change_status=active`
2. set all governed runtime `Artifacts[*].State` to `frozen` for archived lifecycle snapshot
3. migrate runtime/artifact/admission files to archive targets
4. persist final governed lifecycle `change_status=done` in archived change record

#### Scenario: Missing level_source blocks archive
- **WHEN** governed completion archive is requested and `level_source` is missing
- **THEN** archive SHALL be blocked with a clear error

#### Scenario: Archive migrates linked admission snapshot
- **WHEN** governed completion archive is approved and linked admission is `sealed_handoff`
- **THEN** runtime admission file SHALL be migrated to `.spln/archive/admissions/<request_id>.yaml` using governed runtime change state `request_id` as canonical link source

#### Scenario: Governed done archive persists frozen artifact states
- **WHEN** governed `spln done` archive migration completes
- **THEN** archived governed change state SHALL persist all `Artifacts[*].State=frozen`

#### Scenario: Direct-lane archive by request key
- **WHEN** `spln done` is run without a governed change
- **THEN** runtime admission file SHALL be migrated to `.spln/archive/admissions/<request_id>.yaml` using admission `request_id` as canonical archive key

### Requirement: Cancel Auto-Archive Path
`spln cancel` SHALL archive immediately after terminal cancellation.

Cancel-archive sequence:
1. set lifecycle status to terminal cancelled in active lane state
2. archive using request-scoped target resolution
3. preserve cancelled status in archived record

In-flight cancellation preemption contract:
- if wave subprocesses are active, runtime SHALL record cancel-preemption attempt and send `SIGINT`
- runtime SHALL wait up to `execution.cancel_grace_period_seconds`
- subprocesses still alive after grace SHALL be force-terminated via `SIGKILL` before archive migration

Pre-migration status assertion:
- before archive migration starts, persistence layer SHALL observe terminal cancelled status in the active source record
- governed cancel path SHALL observe `change_status=cancelled` in runtime change state
- direct-lane cancel path SHALL observe `admission_status=cancelled` in runtime admission state

Request-scoped targets:
- if governed change exists for current `request_id`:
  - set all governed runtime `Artifacts[*].State` to `frozen` before writing archived governed record
  - move governed runtime change state to `.spln/archive/changes/<request_id>.yaml`
  - move governed artifact bundle to `aircraft/changes/archived/<slug>/`
  - migrate linked `sealed_handoff` admission snapshot to `.spln/archive/admissions/<request_id>.yaml`
- otherwise:
  - move direct-lane runtime admission state to `.spln/archive/admissions/<request_id>.yaml`

Cancel archive MUST NOT require governed `change_status=active`; terminal `change_status=cancelled` is valid for this path.

#### Scenario: Governed cancel accepts terminal cancelled status for archive
- **WHEN** `spln cancel` is invoked for governed request after lifecycle becomes `change_status=cancelled`
- **THEN** archive SHALL proceed using governed request-scoped targets without requiring `change_status=active`

#### Scenario: Cancel writes terminal status before migration
- **WHEN** `spln cancel` is executed
- **THEN** runtime source record SHALL first persist terminal cancelled status, and only then migrate to archive target

#### Scenario: Direct cancel preserves cancelled status in archive
- **WHEN** `spln cancel` is invoked for direct lane request
- **THEN** archived admission record SHALL keep `admission_status=cancelled`

### Requirement: Evidence Schema Persistence
Governance evidence records SHALL persist core fields:
- `request_id`
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

Write-minimization guidance:
- `mitigation_target` is optional denormalized metadata and SHOULD be omitted by default in MVP
- when omitted, consumers SHALL derive mitigation mapping from `skill_name`
- when present, value MUST match registered skill-to-mitigation mapping

`run_summary_version` value domain:
- pre-run-summary governance evidence (`S1_ANALYZE`, `S3_SCOPE_CONFIRMATION`, `S5_PLAN_AUDIT`) SHALL use `run_summary_version=0`
- run-summary-bound governance evidence (`S6_RUN_WAVES`, `S7_REVIEW`, `S8_VERIFY`) SHALL use `run_summary_version>=1`
- review/verify readiness checks SHALL compare evidence version against latest frozen run summary version

Conditional requiredness:
- run-summary-bound governance evidence (`S6/S7/S8`) SHALL include non-empty `input_hash`
- pre-run-summary governance evidence (`S1/S3/S5`) MAY omit `input_hash` in MVP

`session_id` format contract:
- SHALL be UUIDv7 lowercase canonical string

Actor identity contract:
- `actor_id` is optional and, when present, SHALL be stable actor identity for audit correlation
- `role` is optional and, when present, SHALL be one of `implementer|reviewer|operator`

`input_hash` persistence contract:
- when present/required, value SHALL be lowercase hex SHA-256
- when present/required, value SHALL be computed from canonical JSON payload defined by skill-contracts (`request_id`, `state`, `run_summary_version`, normalized `input_scope`, relevant fingerprints/identifiers)

Mitigation mapping contract:
- `mitigation_target` is optional denormalized metadata
- when present, value MUST match registered skill-to-mitigation mapping
- when absent, readiness checks SHALL derive mitigation mapping from `skill_name`

#### Scenario: Missing session_id evidence rejected
- **WHEN** governance evidence lacks `session_id`
- **THEN** evidence validation SHALL fail for readiness checks

#### Scenario: Missing request_id evidence rejected
- **WHEN** governance evidence lacks `request_id`
- **THEN** evidence validation SHALL fail for readiness checks

#### Scenario: Missing run_summary_version evidence rejected
- **WHEN** governance evidence lacks `run_summary_version` for run-summary-bound readiness checks
- **THEN** evidence validation SHALL fail for readiness checks

#### Scenario: Missing input_hash for run-summary-bound evidence rejected
- **WHEN** run-summary-bound governance evidence (`S6/S7/S8`) lacks `input_hash`
- **THEN** evidence validation SHALL fail for readiness checks

#### Scenario: Invalid pre-run-summary run_summary_version is rejected
- **WHEN** pre-run-summary governance evidence is persisted with `run_summary_version>0`
- **THEN** evidence validation SHALL fail for readiness checks

#### Scenario: Missing mitigation target evidence uses derived mapping
- **WHEN** governance evidence lacks `mitigation_target` and `skill_name` is valid
- **THEN** readiness checks SHALL derive mitigation mapping from registry and continue

### Requirement: Evidence Retention and GC
Evidence storage SHALL be bounded by retention policy.

Policy:
- `execution.evidence_retention_days` defines expiry window for GC eligibility
- full/scheduled retention GC is executed by explicit `spln repair` (except low-disk opportunistic one-pass trigger)
- evidence linked to active requests SHALL NOT be deleted by retention GC
- low-disk opportunistic trigger threshold is `execution.evidence_gc_low_disk_free_mb`
- evidence-writing flows SHALL run one opportunistic retention-GC pass when free disk is below threshold

#### Scenario: Retention GC prunes expired non-active evidence
- **WHEN** `spln repair` runs and evidence is older than configured retention window and not linked to active requests
- **THEN** expired evidence MAY be removed and reported in repair summary

#### Scenario: Low disk triggers opportunistic GC pass
- **WHEN** evidence-writing flow detects free disk below `execution.evidence_gc_low_disk_free_mb`
- **THEN** runtime SHALL run one opportunistic retention-GC pass before retrying evidence write

### Requirement: Project Root Discovery
Commands requiring project context SHALL locate root by upward search for `.spln/`.

#### Scenario: Root found from nested path
- **WHEN** command runs in `repo/sub/dir`
- **THEN** root discovery SHALL resolve parent containing `.spln/`

#### Scenario: Root not found
- **WHEN** no `.spln/` exists in parents
- **THEN** command SHALL fail with `spln init` remediation hint
