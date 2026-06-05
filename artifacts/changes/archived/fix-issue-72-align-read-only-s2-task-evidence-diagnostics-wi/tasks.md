# Tasks

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add command-surface regression for S2 plan-drifted task evidence.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/progression_next_test.go"]
  - task_kind: test
  - acceptance: next, validate, and status JSON include task-plan drift, omit wave-orchestration run-summary-missing, and do not create execution-summary.yaml.
  - covers: [REQ-001, REQ-003, REQ-005, REQ-006]

- [x] `t-02` Extract non-mutating wave execution preview from sync diagnosis.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/progression/wave_sync.go"]
  - task_kind: code
  - acceptance: mutating sync behavior remains covered by existing wave-sync tests while preview returns the same task-evidence blockers without writes.
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-03` Refine S2 read-only readiness blockers with preview diagnostics.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["internal/engine/progression/readiness.go"]
  - task_kind: code
  - acceptance: readiness replaces only misleading wave-orchestration run-summary-missing blockers when preview has specific task-evidence blockers.
  - covers: [REQ-001, REQ-002, REQ-004, REQ-006]

- [x] `t-04` Run focused and full verification gates.
  - wave: 4
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["cmd/progression_next_test.go", "internal/engine/progression/readiness.go", "internal/engine/progression/wave_sync.go"]
  - task_kind: verification
  - acceptance: focused command tests, focused progression tests, full Go tests, build, diff check, and Slipway validate evidence pass.
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
