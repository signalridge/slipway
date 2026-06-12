# Tasks

## Task List

- [x] `t-01` Add failing regression tests for bound-worktree `--notes-file`
  resolution and post-S2 evidence wrong-state guidance.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/evidence_skill_test.go", "cmd/evidence_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Add failing regression tests for skill-handoff versus checkpoint
  resume clarity.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: test
  - covers: [REQ-003]

- [x] `t-03` Implement authoritative workspace notes-file resolution and
  post-S2 evidence remediation helpers.
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-04` Implement clearer skill-handoff action and no-checkpoint
  remediation text.
  - wave: 2
  - depends_on: ["t-02"]
  - target_files: ["cmd/next.go", "cmd/next_context_build.go"]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-05` Run focused command tests and repository-level verification.
  - wave: 3
  - depends_on: ["t-03", "t-04"]
  - target_files: ["cmd/evidence.go", "cmd/next.go", "cmd/next_context_build.go", "cmd/evidence_skill_test.go", "cmd/evidence_test.go", "cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003]
