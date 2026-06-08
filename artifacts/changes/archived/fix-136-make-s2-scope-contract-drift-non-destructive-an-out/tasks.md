# Tasks

## Task List

- [x] `t-01` Add `scopeContractDriftOnly` classifier and make the S2 scope-contract advance gate block drift-only failures non-destructively (preserve wave evidence) while still reopening to S2 for missing task changed-file evidence
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/advance_governed.go", "internal/engine/progression/stale_evidence_recovery.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Update the public guidance for `scope_contract_drift` — enrich the remediation/CommandTemplate and the readiness recovery-guidance diagnostic to describe remove/ignore/rescope and state that recorded wave evidence is preserved
  - wave: 1
  - depends_on: []
  - target_files: ["internal/model/recovery.go", "internal/engine/progression/readiness.go"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-03` Add tests: a unit test for `scopeContractDriftOnly` and an e2e test that `slipway run` with an untracked out-of-scope file blocks with `scope_contract_drift` while preserving `wave-orchestration.yaml` + `execution-summary.yaml`
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: ["internal/engine/progression/scope_contract_gate_test.go", "cmd/scope_contract_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
