## ADDED Requirements

### Requirement: Wave Planning Input Contract
Wave planner SHALL normalize tasks into nodes with fields:
- `task_id`
- `objective`
- `task_kind` (`code|test|doc|ops|other`)
- `depends_on[]`
- `target_files[]`
- `verify_cmd` (optional)
- `autonomous` (default `true`)
- `checkpoint_type` (optional)

`task_kind` semantics in MVP:
- `code|test|doc|ops` are reporting/context labels and SHALL NOT change scheduling logic
- only `task_kind=other` has special scheduling behavior

Input sources:
- L2/L3: governed `tasks.md`
- L1: direct brief synthesized into one or more nodes

#### Scenario: L1 synthetic node
- **WHEN** route is L1 and no governed `tasks.md` exists
- **THEN** planner SHALL synthesize deterministic node(s) from direct brief

#### Scenario: Non-`other` task kind does not alter scheduling
- **WHEN** two tasks differ only by `task_kind` among `code|test|doc|ops`
- **THEN** scheduling outcome SHALL be determined by dependency/conflict rules, not by kind label

### Requirement: Dependency Layering and Conflict Split
Wave builder SHALL:
1. build DAG from `depends_on`
2. compute topological layers
3. split same-layer tasks by `target_files[]` conflict sets
4. emit ordered waves

Within each emitted wave, task order SHALL be deterministic by lexical `task_id`.

#### Scenario: Conflict split in same layer
- **WHEN** two same-layer tasks target overlapping files
- **THEN** planner SHALL place them in different waves

### Requirement: Conservative Handling for Unknown Targets
Tasks with empty `target_files[]` SHALL be treated as unknown write scope and isolated into single-task waves.

#### Scenario: Empty target set isolation
- **WHEN** task has no explicit target files
- **THEN** planner SHALL not parallelize it with peer tasks in the same layer

### Requirement: `task_kind=other` Execution Behavior
Tasks with `task_kind=other` SHALL follow manual-path behavior:
- planner SHALL isolate each `other` task into its own wave
- execution SHALL require explicit human checkpoint before final pass verdict
- `other` tasks SHALL NOT run in parallel with non-`other` tasks

#### Scenario: Other task requires manual checkpoint
- **WHEN** a task is marked `task_kind=other`
- **THEN** workflow SHALL pause for explicit user confirmation before marking it `pass`

### Requirement: Wave Execution Contract
`S6_RUN_WAVES` SHALL execute waves sequentially.

Inside one wave:
- parallel execution when `execution.parallelization=true`
- sequential execution when `execution.parallelization=false`

Each task run SHALL use fresh execution context.

#### Scenario: Parallel in-wave execution
- **WHEN** wave contains non-conflicting tasks and parallelization is enabled
- **THEN** tasks SHALL run concurrently

### Requirement: Post-Wave Conflict Verification
After wave execution, runtime SHALL compare emitted `changed_files[]`.

If same-wave tasks overlap on changed files:
- mark involved tasks as `blocked`
- record reason `post_wave_file_conflict`
- stop auto progression for current wave
- require serialized retry path

#### Scenario: Runtime overlap detected
- **WHEN** overlapping `changed_files[]` are found after a wave
- **THEN** affected tasks SHALL be downgraded to `blocked`

### Requirement: Task Output Contract
Each task run SHALL persist:
- `task_id`
- `changed_files[]`
- `test_summary`
- `verify_cmd` result (if provided)
- `commit_ref` (optional)
- `verdict`

Valid verdicts:
- `pass`
- `fail`
- `blocked`
- `timeout`
- `incomplete`
- `cancelled`

#### Scenario: Timeout verdict
- **WHEN** task exceeds timeout budget
- **THEN** verdict SHALL be `timeout`

### Requirement: Spot Checks on Claimed Pass
For tasks claiming `pass`, orchestrator SHALL run lightweight spot checks:
- required output fields are present
- referenced files/commit identifiers are resolvable when provided

Spot-check failure SHALL downgrade `pass` to `incomplete`.

#### Scenario: Missing required output field
- **WHEN** task claims pass but required output field is missing
- **THEN** verdict SHALL be downgraded to `incomplete`

### Requirement: Non-Pass Decision Loop
If any task is non-pass, workflow SHALL pause and require one decision:
- `retry`
- `skip`
- `abort_wave`
- `pivot`

`pivot` SHALL transition analyze-first (`S6/S7/S8 -> S1`) before reroute/rescope handling.

#### Scenario: Retry failed tasks only
- **WHEN** operator selects retry
- **THEN** only non-pass tasks in current wave SHALL rerun

### Requirement: Retry Guard
Wave execution SHALL enforce `execution.max_retries_per_task` (default `2`).

When retry budget is exhausted for a task:
- additional retry SHALL be rejected
- workflow SHALL require `skip`, `abort_wave`, or `pivot`

#### Scenario: Retry budget exhausted
- **WHEN** retry count exceeds `max_retries_per_task`
- **THEN** retry action SHALL be blocked with deterministic remediation

### Requirement: Checkpoint and Continuation
Checkpoint/non-autonomous tasks SHALL pause execution and require explicit user response before resume.

Continuation record SHALL persist:
- paused task id
- checkpoint type
- response payload
- resume timestamp

#### Scenario: Human checkpoint resume
- **WHEN** checkpoint requires human response
- **THEN** execution SHALL resume only after explicit response is recorded

### Requirement: Cancel Preemption
Cancel preemption SHALL terminate active wave subprocesses before archive migration.

When `speclane cancel` targets an active request during wave execution:
- runtime SHALL signal graceful stop (`SIGINT`)
- wait up to configured grace period
- force kill remaining processes (`SIGKILL`)
- persist interrupted task outcomes before archive

#### Scenario: Cancel during active wave
- **WHEN** cancel is invoked while tasks are running
- **THEN** runtime SHALL complete preemption sequence before archive migration

### Requirement: Frozen Run Summary
Wave execution SHALL write frozen run summary snapshots into run record (`.speclane/runs/<request_id>.yaml`).

Snapshot fields SHALL include:
- `summary_version` (monotonic, starts at `1`)
- completed tasks
- non-pass tasks
- carried debt
- open blockers
- frozen timestamp

Run-record update contract:
- each frozen summary SHALL append into `wave_summaries[]`
- `latest_summary_version` SHALL be updated to appended snapshot version

Review (`S7`) and verify (`S8`) SHALL consume the latest frozen summary snapshot, not mutable in-flight buffers.

#### Scenario: New retry creates new snapshot
- **WHEN** a retry completes with updated outcomes
- **THEN** a new `summary_version` snapshot SHALL be appended and selected as latest
