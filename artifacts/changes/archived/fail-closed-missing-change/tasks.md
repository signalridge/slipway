# Tasks

## Task List

- [x] `t-01` Add black-box validate command tests for explicit missing,
  explicit archived, and unscoped no-active behavior.
  - depends_on: []
  - target_files: [cmd/validate_readonly_test.go]
  - task_kind: test
  - covers: [REQ-001]

- [x] `t-02` Update explicit change resolution so true missing explicit slugs
  return `change_not_found` while preserving archived and corrupted-authority
  fail-closed behavior.
  - depends_on: [t-01]
  - target_files: [cmd/common.go, cmd/common_test.go, cmd/resolve_explicit_change_authority_test.go]
  - task_kind: code
  - covers: [REQ-001]
