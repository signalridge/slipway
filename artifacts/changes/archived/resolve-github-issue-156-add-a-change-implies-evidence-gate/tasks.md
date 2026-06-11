# Tasks

## Task List

- [x] `t-01` Add sensitive-evidence evaluator regressions.
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/sensitiveevidence/evaluate.go, internal/engine/sensitiveevidence/evaluate_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `t-02` Implement sensitive-evidence evaluator and readiness wiring.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/engine/sensitiveevidence/evaluate.go, internal/engine/progression/readiness.go, internal/engine/progression/readiness_test.go, internal/engine/progression/advance_governed.go, internal/engine/progression/stale_evidence_recovery.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]

- [x] `t-03` Add reason-code and recovery contract coverage.
  - wave: 3
  - depends_on: [t-02]
  - target_files: [internal/model/reason_code.go, internal/model/reason_code_contract_test.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-004, REQ-007]

- [x] `t-04` Refresh codebase map and run verification.
  - wave: 4
  - depends_on: [t-03]
  - target_files: [artifacts/codebase/ARCHITECTURE.md, artifacts/codebase/CONCERNS.md, artifacts/codebase/CONVENTIONS.md, artifacts/codebase/INTEGRATIONS.md, artifacts/codebase/STACK.md, artifacts/codebase/STRUCTURE.md, artifacts/codebase/TESTING.md, internal/engine/sensitiveevidence/evaluate_test.go, internal/engine/progression/readiness_test.go, internal/engine/progression/scope_contract_gate_test.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-05` Restore public governance skill-evidence recording.
  - wave: 5
  - depends_on: [t-04]
  - target_files: [cmd/evidence.go, cmd/evidence_skill_test.go, cmd/template_flag_contract_test.go, internal/engine/progression/evidence_digests.go, internal/state/verification.go, internal/state/verification_test.go, internal/toolgen/toolgen.go, internal/tmpl/templates/_partials/command-evidence-body.tmpl]
  - task_kind: code
  - covers: [REQ-008]
