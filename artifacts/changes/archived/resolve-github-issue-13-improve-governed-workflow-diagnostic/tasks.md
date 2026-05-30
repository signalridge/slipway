# Tasks
## Project Context
- Tech Stack: Go CLI
- Conventions: structured JSON command surfaces, focused regression tests, governed Slipway artifacts
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Replace task freshness hash authority with explicit structural execution inputs.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/model/execution_summary.go", "internal/model/model_test.go", "internal/model/types.go", "internal/state/execution_summary.go", "internal/state/execution_summary_test.go", "internal/state/wave_execution.go", "internal/engine/context/context.go", "internal/engine/context/context_test.go", "internal/engine/progression/wave_sync.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-012]
  - evidence: artifact
  - acceptance: execution summaries store structural task freshness inputs, stale diagnostics compare expected/current fields, and hash-only summaries fail closed with regeneration guidance.

- [x] `t-02` Centralize actionable next-skill and review-layer diagnostics.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/readiness.go", "internal/engine/progression/authority.go", "internal/engine/progression/readiness_optimization_test.go", "internal/engine/scopecontract/evaluate.go", "internal/engine/scopecontract/evaluate_test.go", "cmd/next_skill.go", "cmd/next_skill_view.go", "cmd/next.go", "cmd/next_handoff.go", "cmd/review.go", "cmd/review_test.go", "cmd/validate.go", "cmd/run.go", "cmd/progression_next_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-010, REQ-012]
  - evidence: verdict
  - acceptance: `next`, `validate`, and `run --diagnostics` agree on the actionable blocking skill in review state and expose exact required layer tokens plus confirmation-boundary metadata.

- [x] `t-03` Add structured diagnostics for stale causality, operator repair mistakes, lifecycle resume, path authority, repair boundaries, artifact DAG blocking state, and health active-change impact.
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: ["artifacts/changes/archived/ai-agent-install-prompt-and-slug-cap/change.yaml", "cmd/common.go", "cmd/common_test.go", "cmd/freshness_diagnostics.go", "cmd/health.go", "cmd/health_test.go", "cmd/next.go", "cmd/repair.go", "cmd/repair_test.go", "cmd/run.go", "cmd/status.go", "cmd/status_artifact_dag_test.go", "cmd/status_view_build.go", "cmd/status_view_build_test.go", "cmd/validate.go", "internal/engine/progression/readiness.go", "internal/engine/status/view.go", "internal/fsutil/atomic.go", "internal/fsutil/atomic_test.go", "internal/model/change.go", "internal/model/model_test.go", "internal/state/execution_repair.go", "internal/state/health.go", "internal/state/store.go", "internal/state/store_test.go"]
  - task_kind: code
  - covers: [REQ-003, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009, REQ-012]
  - evidence: artifact
  - acceptance: JSON diagnostics include stale source/evidence pairs, first stale cause, downstream evidence chain, timestamps, next actions, operator repair mistake details, authoritative linked-worktree paths, resume lifecycle details, applied vs unrepaired repair entries, artifact blocking flags, and active-change health impact.

- [x] `t-04` Update review templates and operator documentation for the new CLI contracts.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl", "internal/engine/artifact/schemas.yaml", "docs/commands.md", "docs/operator-guide.md", "README.md"]
  - task_kind: code
  - covers: [REQ-004, REQ-010, REQ-011, REQ-012]
  - evidence: checklist
  - acceptance: generated skill prompts and docs describe the exact review tokens, structural freshness fields, no-compatibility behavior, and updated diagnostic fields.

- [x] `t-05` Run focused contract tests and full repo verification.
  - wave: 4
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: ["artifacts/changes/resolve-github-issue-13-improve-governed-workflow-diagnostic/assurance.md", "cmd/health_test.go", "cmd/lifecycle_commands_test.go", "cmd/pivot_execution_test.go", "cmd/progression_next_test.go", "cmd/progression_test.go", "cmd/repair_test.go", "cmd/review_test.go", "cmd/stats_test.go", "cmd/status_artifact_dag_test.go", "cmd/status_view_build_test.go", "internal/engine/governance/runtime_actions_test.go", "internal/engine/progression/advance_test.go", "internal/engine/progression/readiness_optimization_test.go", "internal/engine/progression/wave_sync_test.go", "internal/engine/scopecontract/evaluate_test.go", "internal/engine/status/view_test.go", "internal/engine/wave/wave_test.go", "internal/state/execution_summary_test.go", "internal/state/health_test.go", "internal/state/lifecycle_test.go", "internal/state/local_ignore_test.go", "internal/state/wave_execution_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011, REQ-012]
  - evidence: verdict
  - acceptance: targeted tests prove next-skill agreement, structural freshness comparisons, stale-causality chain/root-cause output, operator repair mistake recovery, review-token output, resume/path/repair/status/health diagnostics; `go test ./...`, `go build ./...`, `go vet ./...` where applicable, and `git diff --check` pass or any exception is recorded with a concrete reason.
