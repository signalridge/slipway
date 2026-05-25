# Tasks

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Bind discovery changes to a default repo-local worktree before intent artifact scaffolding.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/new.go", "cmd/new_test.go", "internal/state/worktree.go"]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: focused test `TestNewDiscoveryChangeBindsDefaultWorktreeBeforeIntentArtifact`
  - acceptance: New discovery changes record `.worktrees/<slug>` before `intent.md` creation, with safe skip behavior for no-head/non-Git repos.

- [x] `t-02` Populate missing or scaffold-only codebase-map docs with deterministic baseline repository facts.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/artifact/codebase_map.go", "cmd/codebase_map_command_test.go", "cmd/cli_e2e_test.go", "docs/workflow-test-menu.md", "internal/tmpl/templates/skills/context-assembly/references/codebase-map.md"]
  - task_kind: code
  - covers: [REQ-002]
  - evidence: focused codebase-map command and CLI e2e tests
  - acceptance: `slipway codebase-map --json` reports populated docs and downstream stats see fresh baseline context.

- [x] `t-03` Persist and report archived remediation source relationships.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/done.go", "cmd/lifecycle_commands_test.go", "internal/model/change.go"]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: focused test `TestDoneReportsAndPersistsRemediationSources`
  - acceptance: `done --json` and archived `change.yaml` identify source archived bundles.

- [x] `t-04` Add targeted stale-evidence classification and keep assurance-only edits out of execution freshness.
  - wave: 2
  - depends_on: [t-03]
  - target_files: ["internal/state/execution_summary.go", "internal/state/execution_summary_test.go", "internal/model/reason_code.go", "cmd/review_test.go", "cmd/validate_artifact_gate_test.go", "cmd/status_render_test.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: focused execution-summary, review, validate, status, and done tests
  - acceptance: Planning drift reports `stale_planning_evidence`; execution drift still fails closed; assurance-only edits do not stale execution evidence.

- [x] `t-05` Thin generated root catalog artifacts to metadata and instruction-authority pointers.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go"]
  - task_kind: code
  - covers: [REQ-005]
  - evidence: focused toolgen catalog tests and generated `.codex`/`.claude` catalog spot checks
  - acceptance: Generated catalog artifacts contain `## Instruction Authority` and no `## Full Instructions`.

- [x] `t-06` Update archived workflow feedback dispositions with concrete evidence pointers.
  - wave: 3
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: ["artifacts/changes/archived/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/workflow-feedback.md"]
  - task_kind: doc
  - covers: [REQ-006]
  - evidence: feedback table rows cite changed code/tests and governed evidence
  - acceptance: No actionable row remains marked unresolved, partially fixed, or deferred without explicit rationale.

- [x] `t-07` Complete focused, full, and build verification.
  - wave: 4
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06]
  - target_files: ["artifacts/changes/resolve-workflow-feedback-from-archived-clinvoker-end-to-end-run/verification/goal-verification.yaml", "artifacts/changes/resolve-workflow-feedback-from-archived-clinvoker-end-to-end-run/assurance.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - evidence: focused tests, `go test -timeout=20m ./... -count=1`, `go build ./...`, lifecycle status, and done/archive output
  - acceptance: Focused/full/build checks have captured evidence and downstream S3/S4 gate evidence can verify lifecycle completion without requiring future-state artifacts during S2 execution.
