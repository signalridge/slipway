# Requirements

## Requirements

### Requirement: Dispatch token literal alignment (C0)
REQ-001: The engine MUST recognize the host-written dispatch token value `parallel_subagents` as the valid parallel dispatch mode, and `WaveRun.dispatch_mode` MUST be emitted as `parallel_subagents` (not `parallel`) so the engine reads the exact evidence the generated wave-orchestration skill instructs the host to write.

#### Scenario: Host-written parallel_subagents token is parsed
GIVEN a wave-orchestration verification reference `dispatch_mode:wave=1:parallel_subagents`
WHEN execution sync collects dispatch modes
THEN wave 1's dispatch mode is recognized as the valid parallel mode and is not silently dropped

#### Scenario: Degraded sequential remains valid
GIVEN a reference `dispatch_mode:wave=1:degraded_sequential`
WHEN execution sync collects dispatch modes
THEN wave 1's dispatch mode is `degraded_sequential` and is accepted as valid

### Requirement: Changed-file scope-escape audit (C1)
REQ-002: The engine MUST block when any task's recorded `changed_files` contains a path not covered by any of that task's `target_files`, using the same coverage semantics as the wave planner's conflict detection, with both sides normalized as public paths.

#### Scenario: Out-of-scope changed file is blocked
GIVEN a task whose `target_files` is `internal/a.go` and whose `changed_files` includes `internal/b.go`
WHEN governed wave execution is evaluated
THEN an open blocker `task_changed_file_scope_escape:<task_id>:internal/b.go` is surfaced

#### Scenario: Directory/glob target covers the changed file
GIVEN a task whose `target_files` is `internal/engine/` and whose `changed_files` includes `internal/engine/wave/wave.go`
WHEN governed wave execution is evaluated
THEN no scope-escape blocker is surfaced for that file

### Requirement: Parallel-wave changed-file overlap audit (C1)
REQ-003: For parallel waves only, the engine MUST block when more than one task in the same wave records the same changed file, because same-worktree parallel executors that write the same path can clobber each other.

#### Scenario: Overlap inside a parallel wave is blocked
GIVEN a parallel wave with task A and task B both recording `changed_files` including `internal/x.go`
WHEN governed wave execution is evaluated
THEN an open blocker `parallel_wave_changed_file_overlap:<wave_index>:internal/x.go:<A,B>` is surfaced

#### Scenario: Sequential waves sharing a file are allowed
GIVEN two sequential (non-parallel) waves whose tasks both record `changed_files` including `internal/x.go`
WHEN governed wave execution is evaluated
THEN no overlap blocker is surfaced

### Requirement: Dispatch evidence fail-closed (C2)
REQ-004: The engine MUST NOT infer a parallel dispatch mode for a started parallel wave that lacks an explicit valid `dispatch_mode` token; instead it MUST surface a blocker. A `degraded_sequential` token MUST be accepted without blocking.

#### Scenario: Started parallel wave missing dispatch evidence is blocked
GIVEN a parallel wave that has at least one task evidence record but no valid `dispatch_mode` token
WHEN governed wave execution is evaluated
THEN an open blocker `dispatch_mode_absent_on_started_parallel_wave:<wave_index>` is surfaced and the engine does not record `dispatch_mode` as parallel

#### Scenario: Degraded sequential declaration is not blocked
GIVEN a started parallel wave with a `dispatch_mode:wave=N:degraded_sequential` reference
WHEN governed wave execution is evaluated
THEN no dispatch-evidence blocker is surfaced for that wave

### Requirement: Executor-agent handle validation (C3)
REQ-005: For a wave whose dispatch mode is `parallel_subagents`, the engine MUST require exactly one `executor_agent` handle for each planned task in that wave and block on any missing handle. Waves dispatched `degraded_sequential` and non-parallel waves MUST NOT require handles.

#### Scenario: Missing executor handle on a parallel_subagents wave is blocked
GIVEN a `parallel_subagents` wave with planned tasks t-01 and t-02 but only one recorded `executor_agent` handle (for t-01)
WHEN governed wave execution is evaluated
THEN an open blocker `executor_agent_missing:<wave_index>:t-02` is surfaced

#### Scenario: Degraded sequential wave requires no handles
GIVEN a wave dispatched `degraded_sequential`
WHEN governed wave execution is evaluated
THEN no `executor_agent_missing` blocker is surfaced

### Requirement: Wave-narrowing advisories (C5)
REQ-006: The engine MUST surface non-blocking wave-narrowing advisories in the wave-plan view (view-only, excluded from `wave-plan.yaml` and freshness hashes) for conservative high-confidence signals — a task with directory/glob `target_files` (`broad_target_files`) and a fully serial plan caused by a linear depends_on chain (`fully_serial_plan`) — so plan-audit can cite concrete evidence when rejecting narrative dependencies or over-broad targets.

#### Scenario: Broad target and linear chain are reported as advisories
GIVEN a tasks plan where one task's `target_files` is a directory and every non-root task forms a single linear depends_on chain
WHEN the wave-plan view is produced
THEN `wave_plan.advisories` includes `broad_target_files:<task_id>` and `fully_serial_plan`

#### Scenario: Honest serialization is not reported
GIVEN a plan that serializes only because tasks share concrete file targets (pure file conflict)
WHEN the wave-plan view is produced
THEN no `fully_serial_plan` advisory is surfaced

### Requirement: Generated surfaces and engine boundary aligned (C4)
REQ-007: The generated wave-orchestration skill and docs MUST declare the four engine-enforced blockers and state that, under shared-worktree fan-out, accurate `target_files` plus exhaustive `changed_files` are the safety model, without claiming the engine spawns or executes work itself.

#### Scenario: Generated host surface documents the gates without false claims
GIVEN the regenerated wave-orchestration skill and executor-dispatch reference
WHEN their content is inspected
THEN they name the four blockers and the target/changed-file safety model, and contain neither an engine-spawn claim nor the forbidden markers `engine rejects` / `engine-level rejection`
