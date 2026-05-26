# Tasks

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add narrow test timing seams for command lock/preemption waits and
  set fast package-wide test defaults.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/locks.go", "cmd/lock_helpers.go", "cmd/process_unix.go", "cmd/*_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004]

- [x] `t-02` Delete or consolidate redundant command-surface consistency tests
  while preserving representative lifecycle/governance/readiness coverage.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/governance_gate_consistency_test.go", "cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/health_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004]

- [x] `t-03` Convert safe remaining cwd-based command tests to root-injected
  execution and add `t.Parallel()` where isolated.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/*_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004]

- [x] `t-04` Re-measure targeted and full-suite runtime.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["artifacts/changes/optimize-test-runtime-and-prune-redundant-tests/verification/runtime-timing.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-003, REQ-005]

- [x] `t-05` Run final build, test, and Slipway validation evidence.
  - wave: 4
  - depends_on: [t-04]
  - target_files: ["artifacts/changes/optimize-test-runtime-and-prune-redundant-tests/verification"]
  - task_kind: verification
  - covers: [REQ-005]
