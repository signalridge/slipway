# Tasks

## Task List

- [x] `t-01` Add the shared parsed decision contract and status taxonomy.
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/artifact/decision_contract.go, internal/engine/artifact/decision_contract_test.go, internal/engine/artifact/manager.go, internal/engine/artifact/manager_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - evidence: targeted artifact parser tests pass
  - acceptance: parsed decision status, missing status compatibility, dead status rejection, and normalization coverage are implemented

- [x] `t-02` Wire dead-decision blockers into planning readiness and canonical recovery diagnostics.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/engine/progression/validation.go, internal/engine/progression/validation_test.go, internal/model/reason_code.go, internal/model/reason_code_contract_test.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004]
  - evidence: progression and model tests pass with new reason-code coverage
  - acceptance: superseded, deprecated, rejected, and unknown explicit statuses produce actionable readiness blockers

- [x] `t-03` Reuse the parsed decision contract in next-skill constraints.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/next_skill.go, cmd/next_skill_constraints_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - evidence: next-skill constraint tests pass
  - acceptance: dead decisions produce no pending or locked decision constraints while live or status-free decisions keep existing routing

- [x] `t-04` Refresh governed verification for issue #163 through done-ready.
  - wave: 3
  - depends_on: [t-02, t-03]
  - target_files: [artifacts/changes/resolve-issue-163-decisions-gate/verification, artifacts/changes/resolve-issue-163-decisions-gate/assurance.md]
  - task_kind: verification
  - covers: [REQ-004]
  - evidence: full test suite, validate output, review evidence, goal-verification, and final-closeout records pass
  - acceptance: Slipway reports the change ready for done without running finalization
