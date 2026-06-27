# Tasks

## Task List

- [x] `t-01` Add failing coverage-gate tests for public-surface target integrity
  and actionable diagnostics.
  - depends_on: []
  - target_files: [internal/coverage/coverage_test.go, internal/coverage/cmd/covergate/main_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Implement target-aware coverage baselines and public-surface
  package/file/surface diagnostics.
  - depends_on: [t-01]
  - target_files: [internal/coverage/coverage.go, internal/coverage/cmd/covergate/main.go, coverage-public-surface-baseline.json]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004]

- [x] `t-03` Wire the public-surface gate into CI, local recipes, workflow
  contract tests, and contributor docs.
  - depends_on: [t-02]
  - target_files: [.github/workflows/ci.yml, justfile, cmd/release_workflow_contract_test.go, docs/contributing.md, docs/ja/contributing.md, docs/zh/contributing.md]
  - task_kind: code
  - covers: [REQ-003, REQ-004]

- [x] `t-04` Verify focused coverage gate behavior and the full repository suite.
  - depends_on: [t-03]
  - target_files: [artifacts/changes/add-a-high-risk-public-lifecycle-surface-coverage-gate-for-o/verification/task-results/t-01-evidence.md, artifacts/changes/add-a-high-risk-public-lifecycle-surface-coverage-gate-for-o/verification/task-results/t-02-evidence.md, artifacts/changes/add-a-high-risk-public-lifecycle-surface-coverage-gate-for-o/verification/task-results/t-03-evidence.md, artifacts/changes/add-a-high-risk-public-lifecycle-surface-coverage-gate-for-o/verification/task-results/t-04-evidence.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
