# Assurance

## Scope Summary

This change delivers Workstream B for `add-task-result-import`: compact
executor task result import through repeatable
`slipway evidence task --result-file <json> --json`, engine-owned task evidence
run boundaries, explicit S3 repair re-execution through
`fix --start-reexecution`, run-version-aware wave-orchestration evidence, and
updated generated guidance/docs/surface manifest.

The delivered CLI keeps manual task evidence flags available only as manual
mode. Result-file mode derives `run_summary_version`, `task_kind`,
`target_files`, `captured_at`, and `freshness_inputs` from Slipway state and
rejects executor-provided ledger fields. Repeated `--result-file` values import
atomically: every result is preflighted first, duplicate task IDs fail closed,
and no task evidence is written if any batch member is invalid.

## Verification Verdict

Pass for run 7. The active execution summary, suite-result proof, selected S3
review records, goal-verification, and scope contract all point to the current
run. `go run . validate --json` was captured before final-closeout and reports
the selected review skills as passing, `scope_contract.status=pass`, and only
the expected assurance/final-closeout blockers before this file was authored.

Fresh command proof:

- `verification/logs/focused-cmd-regression-run7.txt`: focused command
  regression passed.
- `verification/logs/selected-packages-run7.txt`: selected package suite passed.
- `verification/logs/full-suite-run7.txt`: full `go test -count=1 ./...` passed.
- `verification/logs/golangci-lint-run7.txt`: `0 issues.`
- `verification/logs/gen-surface-manifest-run7.txt`: manifest is up to date.
- `verification/logs/git-diff-check-run7.txt`: clean.
- `verification/logs/stub-scan-run7.txt`: no real implementation placeholder
  findings.

## Evidence Index

- `verification/wave-plan.yaml`: active S2 authority for run 7.
- `verification/wave-orchestration.yaml`: run 7 wave evidence, including
  `task_result_batch:run7:recorded_count=7` and task evidence refs for t-01
  through t-07.
- `verification/execution-summary.yaml`: run 7 execution summary with all seven
  tasks passing.
- `verification/suite-result.yaml`: full-suite digest for run 7,
  `sha256:ffc88940fece403f83920f6d370e5b4f9b614bfa02ef934f5fb33818cd84c381`.
- `verification/spec-compliance-review.yaml`: passing R0, scope, negative path,
  decision-fidelity, and stub-placeholder review.
- `verification/code-quality-review.yaml`: passing IR1, toolchain compatibility,
  lint, full-suite, and focused-regression review.
- `verification/independent-review.yaml`: passing independent review for
  correctness, quality, tests, contracts, generated surfaces, and batch-event
  behavior.
- `verification/security-review.yaml`: passing secure-default, input-boundary,
  path-safety, ledger-field, schema-limit, and fail-closed review.
- `verification/goal-verification.yaml`: passing acceptance coverage for
  REQ-001 through REQ-007.

## Requirement Coverage

- REQ-001: Pass. Compact result import and repeatable atomic batch import are
  implemented and dogfooded by run 7 task evidence import.
- REQ-002: Pass. Slipway-owned ledger fields are derived, forbidden in result
  JSON, and mixed manual/result-file flags are rejected.
- REQ-003: Pass. The wave plan owns the active execution `run_summary_version`;
  run 7 task and wave evidence use that version.
- REQ-004: Pass. Mixed, invalid, stale, newer, incomplete, malformed, or
  contradictory task/wave evidence fails closed.
- REQ-005: Pass. Executor `changed_files` remains required for attributed tasks,
  and scope-contract enforcement reports pass.
- REQ-006: Pass. Generated guidance, docs, and the surface manifest teach
  result-file import as the default task evidence path and scope manual flags to
  manual mode.
- REQ-007: Pass. Review repair re-execution advanced from run 6 to run 7, stale
  S3 evidence became invalid, and fresh run 7 evidence was recorded.

## Residual Risks and Exceptions

No blocking residual risk remains.

Known accepted constraints:

- Manual task evidence flags remain available for compatibility but are not the
  default generated agent path. Result-file mode rejects mixing those flags.
- Wave 3 was recorded as degraded sequential dispatch because this host did not
  use parallel executor subagents for the implementation batch. The degraded
  dispatch justification is recorded in `verification/wave-orchestration.yaml`.
- The `artifacts/codebase/*` files present in the worktree are exempt context
  files and are not part of this change's staged delivery surface.

## Rollback Readiness

Rollback is local to this PR. Revert the result-file import parser, batch
transaction path, run-version model/state changes, `fix --start-reexecution`
hook, guidance/template/toolgen/docs updates, and associated tests. Then run
`go test -count=1 ./...` and
`go run ./internal/toolgen/cmd/gen-surface-manifest --check` before shipping the
rollback.

No database migration, external service migration, credential migration,
network integration, or irreversible data operation is introduced.

## Archive Decision

Archive decision: ready after final-closeout records freshness, reviewer
independence, goal-verification reuse, and assurance completeness for run 7.

Active `validate --json` proof was captured before `done`; at that point all
selected S3 reviewers and goal-verification passed, evidence freshness was
fresh, and the only remaining blockers were the expected assurance and
final-closeout controls. Archived bundles are not revalidated as active gate
inputs; the active change state is the authority before `done`.
