# Tasks
## Project Context
- Tech Stack: Go, Bash
- Conventions: surgical script/test changes; fixture contracts verify shipped
  helper behavior
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Shell

## Task List

- [x] `t-01` Split `go list` stdout and stderr in `find-polluter-go.sh`.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - evidence: package list populated only from stdout; stderr re-emitted but never passed to go test
  - acceptance: issue #18 ancestor-module reproduction exits with no test packages found and no go test warning package

- [x] `t-02` Keep hard `go list` failures distinct from empty package results.
  - wave: 2
  - depends_on: ["t-01"]
  - target_files: ["internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh", "internal/toolgen/toolgen_test.go"]
  - task_kind: code
  - covers: [REQ-002]
  - evidence: missing package tree reports go list failed for ./does-not-exist/...
  - acceptance: go test ./internal/toolgen passes

- [x] `t-03` Add deterministic issue #18 fixture coverage.
  - wave: 3
  - depends_on: ["t-01"]
  - target_files: ["internal/toolgen/toolgen_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-003]
  - evidence: fixture creates temp parent module and empty child directory
  - acceptance: fixture asserts go list warning is not treated as a package name

- [x] `t-04` Run targeted verification.
  - wave: 4
  - depends_on: ["t-01", "t-02", "t-03"]
  - target_files: ["artifacts/changes/fix-issue-18-harden-find-polluter-go-go-list-handling/verification"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003]
  - evidence: targeted command result for go test ./internal/toolgen and manual reproduction result
  - acceptance: targeted command exits 0

- [x] `t-05` Run full repository verification.
  - wave: 5
  - depends_on: ["t-04"]
  - target_files: ["artifacts/changes/fix-issue-18-harden-find-polluter-go-go-list-handling/verification"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003]
  - evidence: full go test ./... result
  - acceptance: full command exits 0
