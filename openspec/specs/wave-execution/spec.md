# wave-execution Specification

## Purpose
TBD - created by archiving change go-mvp-openspec-workflow. Update Purpose after archive.
## Requirements
### Requirement: Wave Planning Input Contract
Wave planning SHALL normalize tasks into graph-ready nodes with fields:
- `task_id`
- `objective`
- `task_kind` (`code|test|doc|ops|other`)
- `depends_on[]`
- `target_files[]`
- `verify_cmd` (empty string allowed for L1 synthetic tasks)
- `autonomous` (default true)
- `checkpoint_type` (optional: `decision|human_verify|human_action`)

Input sources:
- L2/L3: `tasks.md` from governed `S4_SPEC_BUNDLE`
- L1: direct brief normalized into synthetic node(s)

#### Scenario: L1 synthetic node
- **WHEN** level is L1 and no `tasks.md` exists
- **THEN** planner SHALL create synthetic normalized task node(s) from direct brief

### Requirement: L1 Synthetic Normalization Contract
When L1 uses direct-brief synthesis, planner SHALL generate deterministic normalized node fields.

Input extraction:
- source text SHALL come from admission request title/brief
- if source contains Markdown checklist/bullet lines, planner SHALL create one node per non-empty bullet line
- otherwise planner SHALL create one single synthetic node

Field mapping:
- `task_id`: `l1-<request_short>-NN` (`NN` is zero-padded sequence starting at `01`)
- `request_short`: deterministic short key = first 8 lowercase chars of `request_id`
- `objective`: normalized bullet text or full direct brief sentence
- `task_kind`: keyword-derived (`code|test|doc|ops`) with fallback `other`
- `depends_on[]`: sequential chain for synthesized multi-node sets (`NN` depends on prior node); empty for single-node case
- `target_files[]`: explicit path-like tokens extracted from direct brief; empty when none are detected
- `verify_cmd`: empty string by default unless deterministic command is explicitly present in direct brief
- `autonomous`: `true` by default
- `checkpoint_type`: `human_verify` when `task_kind=other`; omitted otherwise

#### Scenario: L1 single-node defaults
- **WHEN** L1 direct brief has no bullet decomposition
- **THEN** planner SHALL emit one node `l1-<request_short>-01` with empty `depends_on[]` and deterministic defaults

#### Scenario: L1 checklist synthesis
- **WHEN** direct brief contains three checklist bullets
- **THEN** planner SHALL emit `l1-<request_short>-01..03` with sequential dependency chaining

### Requirement: `task_kind=other` Non-Standard Path
`task_kind=other` SHALL represent tasks that are intentionally outside standard wave automation.

For `other` tasks:
- planner SHALL not force dependency-layered automation
- execution SHALL require explicit manual checkpoint before completion
- output SHALL include `verdict=incomplete` until manual confirmation resolves it
- `other` tasks SHALL be isolated into single-task waves and SHALL NOT run in parallel with non-`other` tasks

#### Scenario: Other kind requires manual checkpoint
- **WHEN** a task is marked `task_kind=other`
- **THEN** wave execution SHALL pause for explicit operator confirmation

### Requirement: Dependency-Derived Wave Builder
Wave builder SHALL:
1. build DAG from `depends_on`
2. compute topological layers
3. split each layer by file-conflict sets (`target_files`)
4. emit ordered waves

This behavior SHALL align with GSD wave orchestration semantics.

Ordering determinism inside a layer:
- MVP SHALL NOT use task priority/weight overrides
- any optional `priority`/`weight` metadata (if present in upstream docs) SHALL be ignored by planner
- planner ordering SHALL be deterministic by:
  1. topological layer index
  2. conflict-group split
  3. lexical ascending `task_id` within each emitted wave
- this rule prevents ambiguous execution ordering when no explicit dependency edge exists

#### Scenario: Dependency layering
- **WHEN** task C depends on A and B
- **THEN** C SHALL be placed in a later wave than A and B

#### Scenario: Conflict split
- **WHEN** tasks in same layer touch the same file
- **THEN** they SHALL be split into different waves

### Requirement: Pre-Execution Static Conflict Partition
Before wave execution starts, planner SHALL perform static conflict partition using `target_files[]`.

Static partition contract:
- normalize each `target_files[]` entry to repo-relative path tokens before intersection
- if two tasks in the same topological layer have non-empty intersecting normalized targets, they SHALL NOT be placed in the same wave
- if a task has empty `target_files[]`, planner SHALL treat it as unknown-write scope and isolate it into a single-task wave (no parallel peer in same wave)
- static partition output SHALL be deterministic and reproducible for the same normalized input set

#### Scenario: Predicted overlap split before execution
- **WHEN** two same-layer tasks have overlapping normalized `target_files[]`
- **THEN** planner SHALL split them into separate waves before execution begins

#### Scenario: Unknown target scope is isolated
- **WHEN** a same-layer task has empty `target_files[]`
- **THEN** planner SHALL isolate it into a single-task wave instead of parallel grouping

#### Scenario: Priority metadata does not override DAG order
- **WHEN** two tasks are dependency-independent but one carries higher external priority metadata
- **THEN** planner SHALL still use deterministic DAG + lexical ordering unless dependency graph changes

### Requirement: Wave Execution Model
`S6_RUN_WAVES` SHALL run waves sequentially.
Inside a wave:
- parallel execution when `parallelization=true`
- sequential execution when `parallelization=false`

Each task execution SHALL use fresh subagent session context.

Conflict detection/resolution contract:
- pre-run conflict split SHALL use normalized repo-relative `target_files[]` intersection
- post-run conflict verification SHALL inspect task `changed_files[]` emitted by task output contract
- if two tasks from the same wave report overlapping `changed_files[]`, execution SHALL:
  1. mark both tasks as non-pass (`blocked`) with reason `post_wave_file_conflict`
  2. emit deterministic remediation to rerun as serialized tasks in next retry cycle
  3. stop automatic progression for current wave and enter non-pass control loop
- this applies even when static `target_files[]` did not predict the overlap

Cancel preemption contract:
- if `spln cancel` targets the active request during in-flight wave execution, runtime SHALL request graceful stop (`SIGINT`) for active task subprocesses
- runtime SHALL wait up to `execution.cancel_grace_period_seconds` for graceful stop
- subprocesses still alive after grace period SHALL be force terminated (`SIGKILL`) before archive migration
- interrupted task outcomes SHALL be persisted as cancelled/aborted evidence before lifecycle archive completes

#### Scenario: Parallel in-wave execution
- **WHEN** two non-conflicting tasks are in one wave and parallelization is enabled
- **THEN** they SHALL execute concurrently

#### Scenario: Runtime overlap conflict is downgraded and serialized
- **WHEN** two tasks in the same executed wave produce overlapping `changed_files[]`
- **THEN** both tasks SHALL be marked `blocked` with `post_wave_file_conflict` and require serialized retry path

#### Scenario: Cancel interrupts in-flight wave tasks
- **WHEN** `spln cancel` is invoked while one or more tasks are still running in the active wave
- **THEN** runtime SHALL perform graceful-then-forced interruption per cancel preemption contract before archive migration

### Requirement: Checkpoint and Continuation
For checkpoint or non-autonomous tasks, execution SHALL pause and request user input, then continue with explicit continuation record.

Continuation SHALL preserve:
- completed tasks
- paused task id
- checkpoint type
- user response payload
- resumed run id

#### Scenario: Human verify checkpoint
- **WHEN** checkpoint type is `human_verify`
- **THEN** execution SHALL pause and resume only after user verdict

Checkpoint decision contract:
- decision entrypoint is `spln do`
- checkpoint payload SHALL include `resume_signal` and optional response options
- in interactive mode, response text MAY be collected via prompt
- in non-interactive mode, resume SHALL require explicit `spln do --resume-response "<text>"`
- response text SHALL be stored in continuation record as `user_response_payload`

### Requirement: Task Output Contract
Each task run SHALL output:
- `run_summary_version`
- `changed_files[]`
- `test_summary`
- `evidence_ref`
- `commit_ref` (when commit mode enabled)
- `verdict`

Valid verdicts:
- `pass`
- `fail`
- `blocked`
- `timeout`
- `incomplete`

Task evidence reference contract:
- `evidence_ref` SHALL resolve to deterministic task evidence path:
  - `.spln/evidence/tasks/<request_id>/rv<run_summary_version>/<task_id>.json`
- if same target path exists for a new record, writer SHALL append `--<n>` suffix before `.json` (`n` starts at `1`) instead of overwrite
- `TaskRun.evidence_ref` SHALL store the resolved final path

#### Scenario: Timeout handling
- **WHEN** task exceeds timeout budget
- **THEN** verdict SHALL be `timeout` and treated as non-pass

#### Scenario: Task evidence path determinism
- **WHEN** the same request/task/run-summary tuple emits evidence
- **THEN** `evidence_ref` SHALL follow deterministic path pattern under `.spln/evidence/tasks/`

### Requirement: Wave Spot Checks
After each wave, orchestrator SHALL verify:
- output contract completeness
- evidence ref resolvability
- commit ref resolvability (if enabled)

Spot-check failures SHALL downgrade claimed pass verdicts.

#### Scenario: Missing evidence downgrade
- **WHEN** task reports pass but evidence ref is missing
- **THEN** verdict SHALL be downgraded to `incomplete`

### Requirement: Non-Pass Control Loop
If any task is non-pass, progression SHALL stop and require one decision:
- `retry`
- `skip`
- `abort_wave`
- `pivot`

#### Scenario: Retry failed tasks only
- **WHEN** retry is chosen for one failed task in wave N
- **THEN** only non-pass tasks in wave N SHALL rerun

Decision/state validity rule:
- response validation SHALL use checkpoint-provided allowed responses
- responses outside allowed values for the active checkpoint SHALL be rejected with expected-value remediation

Pivot decision transition rule:
- `pivot` from `S6/S7/S8` SHALL always transition to `S1_ANALYZE` first
- `G_pivot` SHALL be evaluated after analyze refresh and before applying reroute/rescope transition
- if analyze/reroute changes effective level and `G_pivot` approves, workflow follows reroute path from `S1`
- if effective level remains governed, pivot was invoked from `S6`, operator intent is explicit `rescope`, and `G_pivot` approves:
  - `L2` SHALL transition to `S4_SPEC_BUNDLE`
  - `L3` SHALL transition to `S3_SCOPE_CONFIRMATION` before returning to `S4`
- if operator requests `rescope` from `S7` or `S8`, request SHALL be rejected with remediation to return to `S6` first
- implicit automatic rescope transitions without explicit operator intent are not allowed

### Requirement: Retry Guard
System SHALL enforce configurable `max_retries_per_task` (default `2`).

#### Scenario: Retry budget exhausted
- **WHEN** retries exceed max budget
- **THEN** retry SHALL be disallowed and alternatives SHALL be shown

### Requirement: Run-to-Review Handshake
Wave execution SHALL emit run summary for review, including:
- `run_summary_version` (monotonic per request, starts at `1`)
- completed tasks
- non-pass tasks
- carried debt
- evidence set
- open blockers

Before review, latest run summary SHALL be frozen.
New retries SHALL create new summary versions.

Frozen-summary persistence contract:
- persisted path: `.spln/evidence/runs/<request_id>/rv<run_summary_version>.json`
- file is immutable once emitted
- lane state `latest_frozen_run_summary_version` SHALL point to the latest frozen summary

Run-summary version contract applies to all lanes (`L1/L2/L3`):
- L1 direct lane SHALL also emit frozen run summaries
- first frozen summary version for a request SHALL be `1`
- each retry/re-run that produces a new frozen summary SHALL increment version by `1`
- L1 lightweight `S7/S8` checks SHALL consume latest frozen run summary version for that request

#### Scenario: Frozen summary for review
- **WHEN** review starts
- **THEN** reviewer SHALL consume immutable run summary snapshot

#### Scenario: Retry increments run summary version
- **WHEN** a retry cycle produces a new frozen summary
- **THEN** new summary `run_summary_version` SHALL be greater than prior frozen summary version for the same request

#### Scenario: L1 run summary version is present
- **WHEN** an L1 request finishes one `S6_RUN_WAVES` execution cycle
- **THEN** emitted task runs and frozen summary SHALL include `run_summary_version>=1` before lightweight `S7/S8` checks

