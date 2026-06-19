# Tasks

## Task List

- [x] `t-01` Add current-worktree binding status routing and regression tests.
  - depends_on: []
  - target_files: ["cmd/status.go", "internal/state/worktree_binding.go", "cmd/delete_test.go", "internal/state/worktree_binding_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Run focused, package, and repository verification and record implementation evidence.
  - depends_on: [t-01]
  - target_files: ["artifacts/changes/fix-status-bound-worktree-orphan-advice/verification/implementation.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002]
