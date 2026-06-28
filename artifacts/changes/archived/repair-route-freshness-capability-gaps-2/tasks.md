# Tasks

## Task List

- [x] `t-01` Add default compact `run --json` host capability regression
  - depends_on: []
  - target_files: [cmd/progression_next_test.go]
  - task_kind: test
  - covers: [REQ-001]

- [x] `t-02` Add blocker-driven review-alignment cross-surface regression
  - depends_on: []
  - target_files: [cmd/progression_next_test.go]
  - task_kind: test
  - covers: [REQ-002]

- [x] `t-03` Repair any command/view projection gap exposed by the focused regressions
  - depends_on: [`t-01`, `t-02`]
  - target_files: [cmd/next.go, cmd/next_handoff.go, cmd/status.go, cmd/status_view_build.go, cmd/validate.go, cmd/progression_next_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
