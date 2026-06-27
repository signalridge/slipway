# Tasks

## Task List

- [x] `t-01` Add the state-read baseline data model, JSON encoding, and regression threshold comparison logic.
  - depends_on: []
  - target_files: [internal/perfbaseline/baseline.go, internal/perfbaseline/baseline_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004]
  - acceptance: `go test ./internal/perfbaseline -count=1` covers JSON round-trip, passing threshold comparisons, and failing threshold comparisons with command-specific diagnostics.

- [x] `t-02` Add the state-read baseline CLI that builds or accepts a binary, prepares the required synthetic fixture, measures required lifecycle command scenarios, and writes measurements.
  - depends_on: [t-01]
  - target_files: [internal/perfbaseline/cmd/state-read-baseline/main.go, internal/perfbaseline/cmd/state-read-baseline/main_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: A local run of `go run ./internal/perfbaseline/cmd/state-read-baseline -mode refresh -out state-read-performance-baseline.json` uses a built binary and writes measurements for root status, bound status, bound next, bound validate, and explicit `--change` scenarios.

- [x] `t-03` Commit an initial state-read baseline artifact and document the repeat/check commands.
  - depends_on: [t-02]
  - target_files: [state-read-performance-baseline.json, docs/operator-guide.md]
  - task_kind: doc
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: `state-read-performance-baseline.json` records fixture dimensions, command metadata, `real/user/sys` timings, and documented refresh/check commands; `docs/operator-guide.md` names the same commands.

- [x] `t-04` Run targeted tests and final repository verification.
  - depends_on: [t-01, t-02, t-03]
  - target_files: [artifacts/changes/add-state-read-performance-baseline/verification/implementation-tests.txt, artifacts/changes/add-state-read-performance-baseline/verification/full-suite.txt]
  - task_kind: verification
  - covers: [REQ-004]
  - acceptance: Verification evidence records targeted `go test ./internal/perfbaseline -count=1`, the baseline refresh/check commands, and final `go test ./... -count=1`.
