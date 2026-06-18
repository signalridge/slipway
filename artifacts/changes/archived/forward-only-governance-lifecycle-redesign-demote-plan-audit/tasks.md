# Tasks

## Task List

- [x] `t-01` Establish regression coverage for the lifecycle and review-batch redesign
  - depends_on: []
  - target_files: ["cmd/**", "internal/**"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009]

- [x] `t-02` Implement the S0/S1/S2/S3/DONE lifecycle, command surface, current-change amendment, S3 task-plan projection, and review-batch behavior
  - depends_on: [t-01]
  - target_files: ["cmd/**", "internal/**"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009]

- [x] `t-03` Align generated templates, command references, documentation, and active governed artifacts
  - depends_on: [t-02]
  - target_files: ["README.md", "docs/**", "internal/tmpl/**", "artifacts/changes/forward-only-governance-lifecycle-redesign-demote-plan-audit/**"]
  - task_kind: doc
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009]

- [x] `t-04` Run focused and full verification for implementation readiness
  - depends_on: [t-03]
  - target_files: ["README.md", "cmd/**", "docs/**", "internal/**", "artifacts/changes/forward-only-governance-lifecycle-redesign-demote-plan-audit/**"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009]
