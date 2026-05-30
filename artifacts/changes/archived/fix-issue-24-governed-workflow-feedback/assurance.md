# Assurance

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
This change resolves issue #24 workflow feedback across repair, runtime
evidence diagnostics, done/archive warnings, and governed review guidance.

## Verification Verdict
Pass. All four requirements have implementation, regression coverage, review
evidence, and fresh command verification.

## Evidence Index
- Focused regression tests:
  `go test ./cmd -run 'TestRepairRebuildsReadyButStaleExecutionSummaryDrift|TestRepairDoesNotRewriteReadyButStaleExecutionSummaryWhenTaskEvidenceInvalid|TestRepairDoesNotRebuildWhenPlanningEvidenceIsStale'`
  passed.
- Focused issue #24 regression suite:
  `go test ./cmd ./internal/engine/progression ./internal/tmpl -run 'TestRepairRebuildsReadyButStaleExecutionSummaryDrift|TestRepairDoesNotRewriteReadyButStaleExecutionSummaryWhenTaskEvidenceInvalid|TestRepairDoesNotRebuildWhenPlanningEvidenceIsStale|TestEvaluateRequiredSkills_FailsClosedWhenRunSummaryBoundSkillHasNoSummary|TestNextS6GovernedBlocksWithoutTaskEvidenceForWaveRunSummary|TestDoneJSONWarnsWhenWorktreeSourceChangesAreUncommitted|TestReviewTemplatesRequireNegativePathAndToolchainEvidence'`
  passed.
- Full test suite: `go test ./...` passed.
- Build: `go build ./...` passed.
- Diff hygiene: `git diff --check` passed.
- Governance: `go run . validate --json` reported fresh execution evidence and
  passing scope contract after execution evidence synchronization.

## Requirement Coverage
- REQ-001: pass. `repair --json` rebuilds ready stale execution summaries from
  current runtime task evidence when planning evidence is not stale. Invalid
  task evidence and planning drift remain unrepaired; invalid evidence does not
  rewrite `execution-summary.yaml`.
- REQ-002: pass. Missing run-summary and task evidence blockers now include the
  runtime task evidence path and required flat JSON fields.
- REQ-003: pass. `done --json` returns worktree dirty warning fields for
  worktree-bound dirty source files without making Git commits mandatory.
- REQ-004: pass. Review templates and template tests require
  requirement-named negative/error path evidence and dependency/toolchain
  compatibility, including MSRV language.

## Residual Risks and Exceptions
No accepted exceptions.

## Rollback Readiness
Changes are confined to CLI behavior, generated skill text, docs, and tests.
Rollback is reverting this patch.

## Archive Decision
Ready after goal-verification evidence and final Slipway validation.
