# Tasks

## Task List

- [x] `t-01` Implement the intake-specific `intent.md` digest boundary.
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Add regression coverage for issue #238 and downstream digest boundaries.
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/progression/stale_evidence_recovery_test.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]
