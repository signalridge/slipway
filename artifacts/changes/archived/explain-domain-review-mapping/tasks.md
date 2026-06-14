# Tasks

## Task List

- [x] `t-01` Add engine-level required-action satisfied-by attribution
  - depends_on: []
  - target_files: ["internal/engine/governance/actions.go", "internal/engine/governance/runtime_actions.go", "internal/engine/governance/runtime_actions_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-003]

- [x] `t-02` Propagate attribution through command JSON surfaces
  - depends_on: ["t-01"]
  - target_files: ["cmd/status.go", "cmd/status_json.go", "cmd/status_json_test.go", "cmd/validate.go", "cmd/governance_surface.go", "cmd/governance_gate_consistency_test.go"]
  - task_kind: code
  - covers: [REQ-002]
