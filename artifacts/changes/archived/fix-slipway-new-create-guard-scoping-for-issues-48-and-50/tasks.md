# Tasks

## Project Context
- Tech Stack: Go CLI
- Conventions: command behavior under `cmd/`, state/worktree authority under
  `internal/state/`, command regressions in `cmd/*_test.go`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add RED regressions for issue #48, issue #50, and same-workspace collision preservation.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/new_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005]

- [x] `t-02` Re-scope `slipway new` create-guard conflict checks to current/prospective workspace authority.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/new.go", "cmd/common.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-03` Verify targeted create-guard behavior after implementation.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["cmd/new_test.go", "cmd/new.go", "cmd/common.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `t-04` Refresh governed artifacts and codebase-map context for the confirmed design.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["artifacts/changes/fix-slipway-new-create-guard-scoping-for-issues-48-and-50/", "artifacts/codebase/"]
  - task_kind: verification
  - covers: [REQ-005]

- [x] `t-05` Run full fresh verification and Slipway closeout gates.
  - wave: 4
  - depends_on: [t-03, t-04]
  - target_files: ["cmd/new_test.go", "cmd/new.go", "cmd/common.go", "artifacts/changes/fix-slipway-new-create-guard-scoping-for-issues-48-and-50/"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
