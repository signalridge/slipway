# Tasks

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -count=1 ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add failing recovery-route and S3/S4 recovery-guidance regressions.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/scope_contract_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-006]

- [x] `t-02` Add pivot gate/precondition parity regressions.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/gate/gate_test.go", "cmd/pivot_validation_test.go", "cmd/cli_e2e_test.go"]
  - task_kind: test
  - covers: [REQ-005]

- [x] `t-03` Implement non-destructive stale-planning recovery in lifecycle advancement.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/progression/advance_governed.go", "internal/engine/progression/stale_planning_recovery.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-04` Surface actionable read-only recovery guidance in `next` JSON/handoff output.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/next_skill_view.go", "cmd/next.go", "internal/engine/progression/readiness.go", "README.md", "CLAUDE.md", "docs/commands.md"]
  - task_kind: code
  - covers: [REQ-002, REQ-007]

- [x] `t-05` Align `EvaluateGPivot` with CLI pivot preconditions.
  - wave: 2
  - depends_on: [t-02]
  - target_files: ["internal/engine/gate/gate.go", "internal/model/reason_code.go", "cmd/pivot_validation.go"]
  - task_kind: code
  - covers: [REQ-005, REQ-007]

- [x] `t-06` Verify recovered evidence ordering, downstream invalidation, and fail-closed task evidence.
  - wave: 3
  - depends_on: [t-03, t-04, t-05]
  - target_files: ["cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/scope_contract_test.go", "internal/engine/progression/readiness.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-003, REQ-004, REQ-006]

- [x] `t-07` Run focused and full verification gates.
  - wave: 4
  - depends_on: [t-03, t-04, t-05, t-06]
  - target_files: ["cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/scope_contract_test.go", "internal/engine/gate/gate_test.go", "cmd/pivot_validation_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
