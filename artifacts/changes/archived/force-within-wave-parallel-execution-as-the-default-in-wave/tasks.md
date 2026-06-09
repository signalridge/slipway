# Tasks

## Task List

- [x] `t-01` Add `Parallel` to `WavePlanWave` and a validated `DispatchMode` (`parallel`|`degraded_sequential`) to `WaveRun`, with Normalize/Validate and unit tests
  - wave: 1
  - depends_on: []
  - target_files: [internal/model/wave_execution.go, internal/model/wave_execution_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-006]

- [x] `t-02` Add a `parallelization` setting to `model.Config` (default forced, accepts `off`) with validation and unit tests
  - wave: 1
  - depends_on: []
  - target_files: [internal/model/config.go, internal/model/config_test.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-03` Stamp the per-wave `Parallel` flag in `MaterializeWavePlanAt` (derived from task count, excluded from freshness-hash inputs), honor the `parallelization` config, harden same-wave static target conflicts for alias/parent-child/case-only overlaps, and test hash stability plus dispatch-evidence fail-open recovery
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [internal/state/wave_execution.go, internal/state/wave_execution_test.go, internal/engine/wave/wave.go, internal/engine/wave/wave_test.go, internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-04` Surface the per-wave `parallel` signal in `slipway next --json` `input_context.wave_plan.waves[]`, with a view test
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/next.go, cmd/next_wave_plan.go, cmd/next_wave_plan_test.go]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-05` Flip the wave-orchestration skill templates from "parallel when supported" to parallel-by-default, add the `degraded_sequential` recording rule and the `parallelization: off` behavior
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-004, REQ-005]

- [x] `t-06` Add a toolgen skill-contract test asserting parallel-by-default + `degraded_sequential`, and verify generated host adapter output from the committed template
  - wave: 3
  - depends_on: [t-05]
  - target_files: [internal/toolgen/toolgen_test.go, internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl]
  - task_kind: code
  - covers: [REQ-003, REQ-004]

- [x] `t-07` Update wave-execution wording in the workflow docs to describe parallel-by-default and the `parallelization` knob
  - wave: 3
  - depends_on: [t-05]
  - target_files: [docs/workflow.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-005]
