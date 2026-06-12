# Tasks

## Task List

- [x] `t-01` Add generated-surface contract tests for issue #184
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/thin_host_content_test.go, internal/toolgen/toolgen_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]

- [x] `t-02` Implement the wave-orchestration executor dispatch contract
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `t-03` Verify generated-surface sync and full issue acceptance
  - wave: 3
  - depends_on: [t-02]
  - target_files: [docs/SURFACE-MANIFEST.json, artifacts/changes/resolve-github-issue-184-add-gsd-style-automatic-subagent-di/verification/wave-orchestration.yaml]
  - task_kind: test
  - covers: [REQ-006]

- [x] `t-04` Add single worktree parallelization contract tests
  - wave: 4
  - depends_on: [t-03]
  - target_files: [internal/tmpl/thin_host_content_test.go, internal/toolgen/toolgen_test.go]
  - task_kind: test
  - covers: [REQ-007, REQ-008, REQ-009, REQ-010]

- [x] `t-05` Implement single worktree dispatch safety and recovery contract
  - wave: 5
  - depends_on: [t-04]
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md]
  - task_kind: code
  - covers: [REQ-007, REQ-008, REQ-009, REQ-010]

- [x] `t-06` Reverify expanded single worktree acceptance
  - wave: 6
  - depends_on: [t-05]
  - target_files: [docs/SURFACE-MANIFEST.json, artifacts/changes/resolve-github-issue-184-add-gsd-style-automatic-subagent-di/verification/wave-orchestration.yaml]
  - task_kind: test
  - covers: [REQ-006, REQ-007, REQ-008, REQ-009, REQ-010]
