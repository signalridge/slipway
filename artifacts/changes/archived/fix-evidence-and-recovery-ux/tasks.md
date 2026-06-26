# Tasks

## Task List

- [x] `t-01` Reject duplicate task-result session IDs during batch import
  - depends_on: []
  - target_files: ["cmd/evidence.go", "cmd/evidence_task_test.go"]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Add recovery guidance and preventive generated-skill boundary for engine-owned verification YAML
  - depends_on: []
  - target_files: ["internal/state/verification.go", "internal/state/verification_test.go", "cmd/validate_readonly_test.go", "internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl"]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-03` Make ready next diagnostics use advancement wording
  - depends_on: []
  - target_files: ["cmd/next.go", "cmd/progression_next_test.go"]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-04` Preserve archived same-slug residue recovery context
  - depends_on: []
  - target_files: ["internal/model/recovery.go", "internal/model/reason_code.go", "internal/model/recovery_test.go", "cmd/status.go", "cmd/common.go", "cmd/orphaned_bundle_unmanaged_worktree_test.go"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-05` Exclude lifecycle bookkeeping from the wave-orchestration wave-plan freshness digest
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: code
  - covers: [REQ-005]
