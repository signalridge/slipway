# Tasks

## Task List

- [x] `t-01` Mark auto-acknowledged checkpoint resolution events and split learn signals
  - depends_on: []
  - target_files: [cmd/run.go, cmd/stage.go, cmd/next_context_build.go, cmd/next.go, cmd/learn.go, cmd/auto_mode_test.go, cmd/learn_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Replace skill auto-softening blocklist with explicit pure-pacing allowlist
  - depends_on: [t-01]
  - target_files: [internal/engine/progression/confirmation_boundaries.go, cmd/next.go, cmd/auto_mode_test.go]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-03` Pin existing run and README auto-mode redline surfaces
  - depends_on: []
  - target_files: [README.md, internal/toolgen/toolgen_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-004]

- [x] `t-04` Pin auto-off non-pacing blocker precedence over handoff pacing
  - depends_on: [t-02]
  - target_files: [cmd/auto_mode_test.go]
  - task_kind: test
  - covers: [REQ-005]

- [x] `t-05` Run focused and full verification
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: [artifacts/changes/harden-execution-auto-mode/verification]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
