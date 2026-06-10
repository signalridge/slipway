# Tasks

## Task List

- [x] `t-01` Add failing contract tests for Claude `PostToolUse` generation and hook pressure behavior
  - wave: 1
  - depends_on: []
  - target_files: [cmd/context_pressure_hook_test.go, internal/toolgen/adapter_contract_test.go, internal/toolgen/toolgen_test.go, internal/tmpl/hooks_behavior_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Implement context-pressure hook generation and classifier behavior
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/root.go, cmd/context_pressure_hook.go, internal/toolgen/toolgen.go, internal/tmpl/templates/hooks/context-pressure-post-tool-use.sh.tmpl]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-03` Run focused Go verification and record execution evidence
  - wave: 3
  - depends_on: [t-02]
  - target_files: [artifacts/changes/resolve-github-issue-152-add-a-live-context-pressure-posttoo/verification/wave-orchestration.yaml]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]
