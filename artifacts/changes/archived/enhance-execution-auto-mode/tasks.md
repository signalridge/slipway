# Tasks

## Task List

- [x] `t-01` Add auto-aware run/stage loop stop behavior for routine command boundaries
  - depends_on: []
  - target_files: [`cmd/run.go`, `cmd/stage.go`, `cmd/next.go`, `cmd/next_skill_view.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Add focused regression tests for bounded auto continuation and hard-stop preservation
  - depends_on: [`t-01`]
  - target_files: [`cmd/auto_mode_test.go`, `cmd/progression_next_test.go`, `internal/engine/progression/confirmation_boundaries_test.go`]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-03` Align public command documentation and generated command text with bounded auto-to-next-real-gate behavior
  - depends_on: [`t-01`]
  - target_files: [`README.md`, `docs/reference/commands.md`, `docs/commands.md`, `docs/zh/reference/commands.md`, `docs/zh/commands.md`, `docs/ja/reference/commands.md`, `docs/ja/commands.md`, `internal/model/config.go`, `internal/model/config_catalog.go`, `internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, `internal/tmpl/templates/_partials/command-implement-body.tmpl`, `internal/tmpl/templates/_partials/command-intake-body.tmpl`, `internal/tmpl/templates/_partials/command-next-body.tmpl`, `internal/tmpl/templates/_partials/command-plan-body.tmpl`, `internal/tmpl/templates/_partials/command-run-body.tmpl`, `internal/tmpl/templates_test.go`]
  - task_kind: doc
  - covers: [REQ-005]

- [x] `t-04` Run focused verification and record implementation evidence
  - depends_on: [`t-01`, `t-02`, `t-03`]
  - target_files: [`artifacts/changes/enhance-execution-auto-mode/tasks.md`, `artifacts/changes/enhance-execution-auto-mode/assurance.md`, `artifacts/changes/enhance-execution-auto-mode/verification/plan-audit-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/plan-audit.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/wave-plan.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/wave-orchestration-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/wave-orchestration.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/spec-compliance-review-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/spec-compliance-review.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/code-quality-review-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/code-quality-review.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/independent-review-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/independent-review.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/security-review-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/security-review.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/ship-verification-notes.md`, `artifacts/changes/enhance-execution-auto-mode/verification/ship-verification.yaml`, `artifacts/changes/enhance-execution-auto-mode/verification/logs/ship-suite.txt`, `artifacts/changes/enhance-execution-auto-mode/verification/task-results/t-01.json`, `artifacts/changes/enhance-execution-auto-mode/verification/task-results/t-02.json`, `artifacts/changes/enhance-execution-auto-mode/verification/task-results/t-03.json`, `artifacts/changes/enhance-execution-auto-mode/verification/task-results/t-04.json`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
