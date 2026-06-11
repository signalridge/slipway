# Tasks

## Task List

- [x] `t-01` Add and unit-test a reusable file-set transaction helper for ordered write/remove operations, rollback, and rollback-failure diagnostics.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/fsutil/transaction.go", "internal/fsutil/transaction_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-005]
  - evidence: artifact: `go test ./internal/fsutil`
  - acceptance: injected write/remove failures restore original file bytes, remove newly-created files, and report rollback-failure paths

- [x] `t-02` Route S1 planning bundle scaffold and S1-to-S2 wave-plan materialization through the file-set transaction boundary, including injected-failure regression coverage.
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/artifact/manager.go", "internal/engine/artifact/transaction_test.go", "internal/engine/progression/advance_governed.go", "internal/engine/progression/advance_transaction_test.go", "internal/state/store.go", "internal/state/wave_execution.go", "internal/state/wave_execution_transaction_test.go", "internal/state/worktree_binding.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-005]
  - evidence: artifact: `go test ./internal/engine/artifact ./internal/engine/progression ./internal/state`
  - acceptance: injected failure after scaffold or wave-plan mutation leaves no partial generated artifact and does not advance lifecycle authority

- [x] `t-03` Route stale-evidence reopen removals and reopened state save through the same transaction boundary, including evidence-restoration regression coverage.
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/progression/stale_evidence_recovery.go", "internal/engine/progression/stale_evidence_recovery_transaction_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: artifact: `go test ./internal/engine/progression`
  - acceptance: injected failure after evidence removal restores verification files and reports any rollback-failure path
