# Tasks

## Task List

- [x] `t-01` Implement structured S4 hashing for current change authority.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Add regression coverage for evidence-ref-only `change.yaml` mutations and meaningful authority drift.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-03` Verify the #185 fix with targeted and repository-wide checks.
  - wave: 2
  - depends_on: ["t-01", "t-02"]
  - target_files: ["internal/engine/progression/evidence_digests.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003]
