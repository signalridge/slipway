# Tasks

## Task List

- [x] `t-01` C0: change `WaveDispatchParallel` value to `parallel_subagents` and update the dispatch-inference assertions that reference the old literal
  - depends_on: []
  - target_files: [internal/model/wave_execution.go, internal/model/wave_execution_test.go, internal/state/wave_execution_test.go]
  - task_kind: code
  - covers: [REQ-001]
- [x] `t-02` C1a: export `wave.TargetCoversPath(targets, file) bool` wrapping the existing `normalizeTargetFileForConflict`/`targetFileContains`/`targetPatternMatches` predicates, with coverage tests
  - depends_on: []
  - target_files: [internal/engine/wave/wave.go, internal/engine/wave/wave_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
- [x] `t-03` C1b: add `TaskChangedFileScopeEscapeBlockers` and `ParallelWaveChangedFileOverlapBlockers`, define their reason codes, and wire them into `evaluateGovernedWaveExecution` under the plan-drift guard
  - depends_on: [t-02]
  - target_files: [internal/engine/progression/wave_sync.go, internal/model/reason_code.go, internal/engine/progression/wave_sync_test.go, internal/model/reason_code_contract_test.go, cmd/lifecycle_commands_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
- [x] `t-04` C2: remove the silent-parallel inference in `waveRunDispatchMode`, add `DispatchEvidenceBlockers` + its reason code, wire it in, and rewrite the inference-dependent state tests
  - depends_on: [t-03]
  - target_files: [internal/state/wave_execution.go, internal/engine/progression/wave_sync.go, internal/model/reason_code.go, internal/state/wave_execution_test.go, internal/engine/progression/wave_sync_test.go, internal/state/repair_test.go, internal/model/reason_code_contract_test.go]
  - task_kind: code
  - covers: [REQ-004]
- [x] `t-05` C3: add `model.ExecutorAgentHandlesFromVerification`, `ExecutorAgentBlockers` + its reason code, and wire it in for `parallel_subagents` waves only
  - depends_on: [t-04, t-01]
  - target_files: [internal/model/wave_execution.go, internal/engine/progression/wave_sync.go, internal/model/reason_code.go, internal/model/wave_execution_test.go, internal/engine/progression/wave_sync_test.go, internal/model/reason_code_contract_test.go]
  - task_kind: code
  - covers: [REQ-005]
- [x] `t-06` C5: add `wave.AnalyzeWaveNarrowingCauses` and a view-only `Advisories []string` on the wave-plan view, populated on the derived and from-model paths, with tests
  - depends_on: [t-02]
  - target_files: [internal/engine/wave/wave.go, cmd/next.go, cmd/next_wave_plan.go, internal/engine/wave/wave_test.go, cmd/next_wave_plan_test.go]
  - task_kind: code
  - covers: [REQ-006]
- [x] `t-07` C4: align the wave-orchestration SKILL template and executor-dispatch reference with the four engine-enforced blockers and the target/changed-file safety model, and regenerate host surfaces
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md, internal/tmpl/wave_isolation_content_test.go]
  - task_kind: code
  - covers: [REQ-007]
