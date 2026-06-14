# Tasks

## Task List

- [x] `t-01` Update shared SessionStart hook template to render cross-worktree active changes as informational handoff context.
  - depends_on: []
  - target_files: [internal/tmpl/templates/hooks/session-start.sh.tmpl]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-02` Add hook behavior regressions for cross-worktree informational output and preserved diagnostics.
  - depends_on: [t-01]
  - target_files: [internal/tmpl/hooks_behavior_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-03` Pin explicit command fail-closed behavior for wrong-worktree `run` / `next` active-change resolution.
  - depends_on: []
  - target_files: [cmd/active_change_resolution_test.go]
  - task_kind: test
  - covers: [REQ-002]

- [x] `t-04` Run focused verification for hook rendering and command fail-closed behavior.
  - depends_on: [t-02, t-03]
  - target_files: [artifacts/changes/resolve-github-issue-212-make-session-start-handoff-treat-cr/verification/wave-orchestration-notes.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
