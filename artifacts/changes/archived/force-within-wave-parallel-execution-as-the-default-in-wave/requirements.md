# Requirements

## Requirements

### Requirement: Explicit per-wave parallel signal
REQ-001: The materialized wave plan MUST mark every wave that contains more than one task with an explicit `parallel` signal, because the engine already guarantees same-wave tasks are dependency-free and file-disjoint.

#### Scenario: Multi-task wave is marked parallel
GIVEN a tasks plan with two independent, file-disjoint tasks in the same dependency layer
WHEN the wave plan is materialized
THEN that wave carries `parallel: true`, and a wave with a single task carries `parallel: false`.

### Requirement: Parallel signal surfaced to the host
REQ-002: `slipway next --json` at `S2_EXECUTE` MUST surface the per-wave `parallel` signal inside `input_context.wave_plan.waves[]` so the host can decide dispatch.

#### Scenario: next --json exposes the signal
GIVEN an active change at `S2_EXECUTE` whose wave plan has a multi-task wave
WHEN `slipway next --json` is run
THEN the corresponding wave object in `input_context.wave_plan.waves[]` reports the `parallel` signal.

### Requirement: Skill instructs parallel-by-default
REQ-003: The generated `wave-orchestration` skill MUST instruct the host to dispatch all tasks in a multi-task wave concurrently by default, rather than presenting parallel dispatch as an optional "when supported" path.

#### Scenario: Generated skill mandates concurrency
GIVEN the wave-orchestration skill regenerated from the templates
WHEN its dispatch section is read
THEN it states that a multi-task wave is dispatched concurrently by default, and names sequential execution only as a recorded fallback.

### Requirement: Dispatch mode is recorded
REQ-004: A wave run MUST record its dispatch mode as `parallel` or `degraded_sequential`, and the `WaveRun` model MUST reject any other value, so that lost parallelism is visible rather than silent.

#### Scenario: WaveRun validates dispatch mode
GIVEN a `WaveRun` value
WHEN its `dispatch_mode` is `degraded_sequential` or `parallel`
THEN validation passes; WHEN it is any other non-empty string THEN validation fails.

### Requirement: Parallelization off-switch
REQ-005: A `parallelization` configuration setting MUST default to forced and MUST accept `off`; when `off`, the wave plan and skill surfaces MUST reflect a non-forced mode so a project can opt out.

#### Scenario: off flips the effective mode
GIVEN a repository configured with `parallelization: off`
WHEN the wave plan is materialized and `slipway next --json` is read
THEN waves carry no forced-parallel signal and the effective mode is non-forced; GIVEN no configured value THEN the effective mode is forced.

### Requirement: Signal does not corrupt freshness
REQ-006: The per-wave `parallel` signal MUST NOT change the wave-plan freshness hashes (`tasks_plan_hash` and the structural/scope/semantic hashes), which derive from `tasks.md`; it MUST be derived at materialize and excluded from hash inputs.

#### Scenario: Hashes are unchanged by the signal
GIVEN a tasks plan
WHEN the wave plan is materialized with the `parallel` signal set
THEN `tasks_plan_hash` equals the hash computed for the same tasks plan independent of the signal.
