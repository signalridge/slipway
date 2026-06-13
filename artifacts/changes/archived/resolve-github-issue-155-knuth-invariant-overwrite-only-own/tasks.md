# Tasks

## Task List

- [x] `t-01` Add failing digest materiality tests for prose artifacts.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Implement prose artifact material-view hashing for skill input digests.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/progression/evidence_digests.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-03` Verify focused and full Go test suites plus Slipway validation.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["internal/engine/progression/evidence_digests.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002]
