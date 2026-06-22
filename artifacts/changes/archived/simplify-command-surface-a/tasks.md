# Tasks

## Task List

- [x] `t-01` Remove checkpoint lifecycle state and command/runtime protocol
  - depends_on: []
  - target_files: ["cmd/abort.go", "cmd/checkpoint.go", "cmd/common.go", "cmd/context_pressure_hook.go", "cmd/health.go", "cmd/lifecycle_events.go", "cmd/next.go", "cmd/next_context_build.go", "cmd/next_handoff.go", "cmd/next_resume_response.go", "cmd/repair.go", "cmd/root.go", "cmd/run.go", "cmd/session_start_hook.go", "cmd/stage.go", "cmd/status_view_build.go", "internal/model/change.go", "internal/model/reason_code.go", "internal/state/health.go", "internal/state/execution_repair.go", "internal/state/store.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - acceptance: `go run . run --help` and `go run . implement --help` no longer show `--resume-response`; context-pressure guidance no longer recommends `slipway checkpoint`; source search shows no live `ActiveCheckpoint`, `resume_checkpoint`, checkpoint-specific reason code, or checkpoint repair/status branch remains in these owned files.
  - evidence: task evidence references command help output plus search transcript for checkpoint lifecycle/protocol removals.

- [x] `t-02` Remove checkpoint metadata from wave/task parsing and resume comments while preserving non-checkpoint resume helpers
  - depends_on: []
  - target_files: ["internal/engine/wave/parse.go", "internal/engine/wave/parse_test.go", "internal/engine/wave/wave.go", "internal/engine/progression/wave_sync.go", "internal/model/wave.go", "internal/model/wave_execution.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - acceptance: task parsing rejects or ignores no live `checkpoint_type` surface, task-plan hashing no longer includes checkpoint metadata, and `BuildResumeCompletedTasks` / resume-index helpers continue to compile and describe non-checkpoint resume behavior.
  - evidence: task evidence references focused `go test ./internal/engine/wave ./internal/engine/progression ./internal/model` output and search transcript for `checkpoint_type` / checkpoint-resume comments.

- [x] `t-03` Remove learn and stats command surfaces while preserving retained diagnostics
  - depends_on: []
  - target_files: ["cmd/learn.go", "cmd/learn_test.go", "cmd/root.go", "cmd/stats.go", "cmd/stats_test.go", "cmd/status.go", "internal/state/stats.go", "internal/state/stats_test.go"]
  - task_kind: code
  - covers: [REQ-002]
  - acceptance: `go run . --help` no longer lists standalone `learn` or `stats`; `learn_apply_unsupported` has no live command path; `go run . status --stats --json` still works or retained diagnostics have a documented replacement.
  - evidence: task evidence references root help output, `status --stats` output, focused `go test ./cmd ./internal/state`, and search transcript for deleted learn/stats surfaces.

- [x] `t-04` Update tests for deleted checkpoint/learn/stats surfaces and preserved resume regressions
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["cmd/abort_test.go", "cmd/auto_mode_test.go", "cmd/checkpoint_test.go", "cmd/cli_e2e_test.go", "cmd/codebase_map_context_test.go", "cmd/command_description_contract_test.go", "cmd/context_pressure_hook_test.go", "cmd/governance_readiness_error_test.go", "cmd/health_test.go", "cmd/lifecycle_commands_test.go", "cmd/next_eval_fixture_test.go", "cmd/progression_next_test.go", "cmd/repair_test.go", "cmd/root_help_test.go", "cmd/status_test.go", "cmd/status_view_build_test.go", "cmd/template_flag_contract_test.go", "internal/model/model_test.go", "internal/model/reason_code_contract_test.go", "internal/model/recovery_test.go", "internal/state/health_test.go", "internal/state/repair_test.go", "internal/state/store_test.go", "internal/engine/progression/advance_governed_test.go", "internal/engine/progression/wave_boundary_evidence_test.go", "internal/engine/progression/wave_sync_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-004, REQ-005]
  - acceptance: tests no longer assert checkpoint command/resume-response behavior in command-description contracts, template flag contracts, governance readiness diagnostics, status view, repair, lifecycle confirmation prose, context-pressure hook output, or command E2E paths; focused tests prove `run --resume` still resumes interrupted/incomplete wave execution or fails closed with existing remediation; blocked/incomplete task evidence has a viable rerun/resume path; retained fail-closed evidence tests still cover stale execution evidence, incomplete execution tasks, malformed task evidence, changed-file scope escape, and parallel changed-file overlap blockers.
  - evidence: task evidence references targeted `go test ./cmd ./internal/model ./internal/state ./internal/engine/progression` output.

- [x] `t-05` Align toolgen, templates, docs, and surface manifest with deleted A surfaces
  - depends_on: [t-01, t-03]
  - target_files: ["docs/SURFACE-MANIFEST.json", "docs/ai-tools.md", "docs/commands.md", "docs/explanation/workflow.md", "docs/index.md", "docs/reference/ai-tools.md", "docs/reference/commands.md", "docs/tutorials/first-governed-change.md", "docs/workflow.md", "internal/toolgen/adapter_contract_test.go", "internal/toolgen/install_profiles.go", "internal/toolgen/install_profiles_test.go", "internal/toolgen/surface_manifest.go", "internal/toolgen/testdata/skill_tree_inventory.codex.golden", "internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go", "internal/tmpl/templates/_partials/command-abort-body.tmpl", "internal/tmpl/templates/_partials/command-checkpoint-body.tmpl", "internal/tmpl/templates/_partials/command-implement-body.tmpl", "internal/tmpl/templates/_partials/command-intake-body.tmpl", "internal/tmpl/templates/_partials/command-learn-body.tmpl", "internal/tmpl/templates/_partials/command-next-body.tmpl", "internal/tmpl/templates/_partials/command-plan-body.tmpl", "internal/tmpl/templates/_partials/command-run-body.tmpl", "internal/tmpl/templates/_partials/command-stats-body.tmpl", "internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl", "internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl", "internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/tmpl/templates/skills/workflow/SKILL.md.tmpl", "internal/tmpl/templates_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: toolgen registry, install profiles, templates, docs, generated skill inventory expectations, and `docs/SURFACE-MANIFEST.json` no longer advertise `checkpoint`, `learn`, `stats`, `$slipway-checkpoint`, `$slipway-learn`, `$slipway-stats`, `--resume-response`, `resume_checkpoint`, or `checkpoint_type` as live guidance.
  - evidence: task evidence references `go test ./internal/tmpl ./internal/toolgen`, `go run ./internal/toolgen/cmd/gen-surface-manifest --check`, and search transcript for generated-surface tokens.

- [x] `t-06` Run targeted verification, surface checks, search checks, and full suite
  - depends_on: [t-04, t-05]
  - target_files: ["artifacts/changes/simplify-command-surface-a/verification/implementation-verification.md", "artifacts/changes/simplify-command-surface-a/tasks.md", "cmd/evidence.go", "cmd/evidence_suite_result_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: capture and summarize these checks: `go test ./cmd ./internal/model ./internal/state ./internal/engine/wave ./internal/engine/progression ./internal/tmpl ./internal/toolgen`, `go run ./internal/toolgen/cmd/gen-surface-manifest --check`, `go run . --help`, `go run . run --help`, `go run . implement --help`, `go run . evidence --help`, `go run . evidence suite-result --help`, the Workstream A search checks from requirements/research, and `go test ./...`.
  - evidence: task evidence references `artifacts/changes/simplify-command-surface-a/verification/implementation-verification.md` containing command outputs or summaries with timestamps and any documented unrelated failures.
