# Tasks

## Project Context
- Tech Stack: Go
- Conventions: Keep code edits scoped to `internal/state`, `internal/toolgen`, related CLI views/tests, and docs touched by removed compatibility behavior.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01-runtime-sidecar-removal` Remove legacy `runtime-state.yaml` compatibility.
  - wave: 1
  - depends_on: []
  - target_files: [`internal/state/change_runtime.go`, `internal/state/store.go`, `internal/state/health.go`, `internal/state/execution_repair.go`, `internal/state/lifecycle.go`, `internal/state/repair.go`, `internal/model/change.go`, `internal/engine/progression/readiness.go`, `cmd/health.go`, `cmd/repair.go`, `cmd/lifecycle_events.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-005]
  - acceptance: no production code loads, diagnoses, repairs, migrates, deletes, or reports `runtime-state.yaml`.
  - evidence: focused state/health/repair tests updated or removed.

- [x] `t-02-toolgen-legacy-cleanup-removal` Remove old generated-surface upgrade cleanup.
  - wave: 1
  - depends_on: []
  - target_files: [`internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, `internal/toolgen/support_files_test.go`, `cmd/init_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-005]
  - acceptance: toolgen no longer carries branches for retired generated artifacts, legacy Codex agent blocks, legacy post-tool hooks, or legacy provenance metadata.
  - evidence: toolgen tests describe only the current first-version generated tree and deterministic refresh behavior.

- [x] `t-03-doc-contract-update` Update docs to remove old compatibility promises.
  - wave: 2
  - depends_on: [t-01-runtime-sidecar-removal, t-02-toolgen-legacy-cleanup-removal]
  - target_files: [`README.md`, `docs/command-contract-matrix.md`, `docs/execution-surface-boundary.md`, `docs/agent-contracts.md`, `docs/README.md`]
  - task_kind: code
  - covers: [REQ-003, REQ-005]
  - acceptance: docs no longer promise `runtime-state.yaml` migration or old generated-surface cleanup.
  - evidence: `rg` shows removed compatibility terms only where historical ADR text remains intentionally preserved.

- [x] `t-04-current-contract-regression` Keep current first-version contracts covered.
  - wave: 2
  - depends_on: [t-01-runtime-sidecar-removal, t-02-toolgen-legacy-cleanup-removal]
  - target_files: [`cmd/*_test.go`, `internal/state/*_test.go`, `internal/toolgen/*_test.go`, `internal/engine/*_test.go`]
  - task_kind: code
  - covers: [REQ-004, REQ-005]
  - acceptance: tests that remain assert current command/JSON/generated-surface behavior rather than old-version compatibility.
  - evidence: focused package tests pass.

- [x] `t-05-full-verification` Run final verification.
  - wave: 3
  - depends_on: [t-03-doc-contract-update, t-04-current-contract-regression]
  - target_files: ["artifacts/changes/remove-obsolete-backward-compatibility-paths-and-redundant-legacy-code-from-slipway/verification/execution-summary.yaml"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: `go test ./...` and `go build ./...` pass.
  - evidence: command outputs recorded in execution summary.
