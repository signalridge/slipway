# Tasks

## Task List

- [x] `t-01` Add failing reason-code taxonomy and unknown-code regressions.
  - wave: 1
  - depends_on: []
  - target_files: [internal/model/reason_code.go, internal/model/model_test.go, internal/model/recovery_test.go, internal/model/reason_code_contract_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Implement explicit canonical lookup and unknown-code normalization.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/model/reason_code.go, internal/model/recovery.go, cmd/status_view_build.go, internal/model/model_test.go, internal/model/recovery_test.go, internal/engine/gate/gate_test.go, internal/engine/progression/evidence.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-03` Add repo-local message-prose assertion lint and migrate caught tests to stable fields.
  - wave: 3
  - depends_on: [t-02]
  - target_files: [internal/model/reason_code_contract_test.go, internal/model/model_test.go, internal/model/recovery_test.go, internal/engine/gate/gate_test.go, internal/engine/progression/evidence_test.go, internal/engine/progression/readiness_optimization_test.go, internal/engine/progression/wave_sync_test.go, internal/state/execution_summary_test.go, internal/state/health_test.go, internal/state/verification_test.go, cmd/active_change_resolution_test.go, cmd/common_test.go, cmd/delete_test.go, cmd/error_contract_test.go, cmd/governance_readiness_error_test.go, cmd/health_test.go, cmd/hydrate_flag_test.go, cmd/lifecycle_commands_test.go, cmd/new_test.go, cmd/progression_next_test.go, cmd/route_surface_command_test.go]
  - task_kind: test
  - covers: [REQ-003]

- [x] `t-04` Run targeted and full verification, then record governed task evidence.
  - wave: 4
  - depends_on: [t-03]
  - target_files: [docs/operator-guide.md, internal/engine/progression/evidence.go, internal/engine/progression/evidence_test.go, internal/model/reason_code.go, internal/model/recovery.go, internal/model/recovery_test.go, internal/model/reason_code_contract_test.go, artifacts/changes/resolve-github-issue-160-freeze-reason-code-taxonomy-with-a/verification/wave-orchestration.yaml]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]
