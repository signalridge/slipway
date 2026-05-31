# Tasks

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add RED regression coverage for execution-summary self-staleness.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/state/execution_summary_test.go"]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: failing then passing targeted `go test -count=1 ./internal/state -run TestExecutionSummaryFreshnessIgnoresSummaryCapturedAtForPerTaskFreshness`
  - acceptance: test models two matching task evidence timestamps and a later summary `captured_at`, and expects fresh diagnostics.

- [x] `t-02` Correct per-task freshness baseline while preserving fail-closed behavior.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/state/execution_summary.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - evidence: targeted `internal/state` tests for #28 and unreadable freshness artifacts.
  - acceptance: normal per-task freshness uses upstream artifacts only; unreadable upstream artifacts still stale the execution summary.

- [x] `t-03` Add validate zero-write regression tests for no-active, archived, and orphan-bundle paths.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/validate_readonly_test.go"]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: targeted `go test -count=1 ./cmd -run 'TestValidate(NoActiveDiagnostic|ArchivedExplicitSlug|OrphanActiveBundle)IsZeroWrite'`
  - acceptance: tests snapshot non-git files and prove `validate --json` does not write on those failure/diagnostic paths.

- [x] `t-04` Clarify active validate contract in docs and templates.
  - wave: 1
  - depends_on: []
  - target_files: ["docs/commands.md", "docs/operator-guide.md", "internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl", "internal/tmpl/templates/artifacts/assurance.md"]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: docs/template diff plus `go test -count=1 ./internal/tmpl`.
  - acceptance: wording says `validate --json` is active pre-`done` readiness proof and archived audit is a separate future surface.

- [x] `t-05` Verify full change and close tracker issues according to evidence.
  - wave: 2
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: ["artifacts/changes/fix-execution-summary-freshness-and-clarify-validate-diagnos/assurance.md", "artifacts/changes/fix-execution-summary-freshness-and-clarify-validate-diagnos/tasks.md", "artifacts/changes/fix-execution-summary-freshness-and-clarify-validate-diagnos/verification/", "artifacts/codebase/ARCHITECTURE.md", "artifacts/codebase/CONCERNS.md", "artifacts/codebase/CONVENTIONS.md", "artifacts/codebase/INTEGRATIONS.md", "artifacts/codebase/STACK.md", "artifacts/codebase/STRUCTURE.md", "artifacts/codebase/TESTING.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: `go test -count=1 ./...`, `go build ./...`, `go run . validate --json`, and GitHub issue comments/closures.
  - acceptance: #28 fix is verified; #29/#30/#32/#34 are closed or commented with evidence-backed reasoning matching the user's final disposition.
