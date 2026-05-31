# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/state/execution_summary.go` owns execution-summary freshness evaluation, diagnostics, planning-drift pairs, runtime task evidence timestamp checks, and active/archived summary path authority.
  - `internal/model/execution_summary.go` owns `ExecutionSummary.LatestRelevantUpdateAt()`, which returns the max of summary `CapturedAt` and task `CapturedAt`; it remains useful for summary coverage/fail-closed paths but must not seed per-task freshness.
  - `cmd/validate.go` resolves active changes before building validation views; the no-active fallback returns diagnostic JSON without entering `buildValidateViewForSlug`.
  - `cmd/common.go` rejects archived explicit slugs with `archived_change_not_validatable`, preserving active-only validate semantics.
  - `internal/state/local_ignore.go` confirms current Slipway HEAD ignores `artifacts/changes/**/{evidence,events,verification}/` and does not generate archived-path negations.
  - `cmd/repair.go` reports non-empty orphan bundles as non-repairable findings rather than deleting them automatically.
- Dependency chains:
  - `validate/status/review/next` -> `LoadRelevantExecutionSummaryContext` -> `ExecutionSummaryFreshnessDiagnostics` -> `executionSummaryFreshnessEvaluation` -> `latestExecutionRelevantUpdateAt` + `collectTaskEvidenceFreshnessInputs`.
  - `validate --json` -> `resolveActiveChangeRef` -> no-active diagnostic or `buildValidateViewForSlug`; archived explicit slugs stop in `resolveExplicitChange`.
- Blast radius: bounded to execution freshness logic, validate read-only regression coverage, and operator-facing docs/templates. No command semantics or archive lifecycle behavior should change.
- Constraints:
  - `execution-summary.yaml` is downstream aggregate evidence.
  - Per-task freshness may compare task evidence to structural inputs and upstream artifacts, but not to the summary's own `CapturedAt` or sibling task timestamps.
  - Planning-source drift still needs fail-closed behavior when an upstream artifact is unreadable.

### Patterns
- Existing conventions:
  - Freshness is expressed through `ctxpack.EvidenceFreshnessInput`, with structural field maps and timestamp comparisons.
  - Current task input diffs already compare task `freshness_inputs` and runtime task evidence `captured_at` before generic timestamp freshness evaluation.
  - Read-only command surfaces have regression tests proving they do not persist artifact reconciliation or governance snapshots.
- Reusable abstractions:
  - Reuse `ExpectedExecutionTaskFreshnessInputs`, `taskEvidenceCapturedAt`, and `ExecutionSummaryFreshnessDiagnostics` for #28 tests.
  - Reuse command test helpers (`commandForRoot`, `createGovernedRequest`, `state.ArchiveChange`) for #32 zero-write tests.
- Convention deviations: none required. The minimal code change keeps existing freshness APIs and narrows only the baseline used for per-task evidence timestamp comparison.

### Risks
- Technical risks:
  - High: removing summary/task timestamps from the per-task baseline could accidentally make unreadable upstream artifacts non-stale. Mitigation: keep a fail-closed helper that uses summary/task evidence timestamps only on error paths.
  - Medium: summary-level blocker-only evidence can become `unknown` if no per-task inputs exist and there is no upstream timestamp. Mitigation: fallback no-task evaluation treats the summary timestamp as its own baseline.
  - Low: docs wording could be over-broad. Mitigation: state only the active vs archived validate contract, not a new audit feature.
- Guardrail domains: none. The change does not modify auth, credentials, PII, financial, schema migration, irreversible operations, or external API contracts.
- Reversibility: straightforward revert of freshness baseline, tests, and docs if regressions appear.

### Test Strategy
- Existing coverage:
  - `internal/state/execution_summary_test.go` covers structural freshness inputs, manual task timestamp drift, planning-source-first diagnostics, task-plan hash mismatch, planning evidence chains, unreadable freshness artifacts, and archived summary sanitization.
  - `cmd/common_test.go` already covers archived explicit slug diagnostics.
  - `cmd/status_context_repair_test.go` already covers validate read-only artifact reconcile.
- New coverage:
  - Add a RED test proving summary `CapturedAt` newer than all task evidence no longer makes matching per-task evidence stale.
  - Add validate zero-write tests for no-active diagnostic fallback, archived explicit slug rejection, and non-empty orphan active bundle residue.
- Verification approach:
  - Targeted RED/GREEN: `go test -count=1 ./internal/state -run TestExecutionSummaryFreshnessIgnoresSummaryCapturedAtForPerTaskFreshness`.
  - Targeted read-only checks: `go test -count=1 ./cmd -run 'TestValidate(NoActiveDiagnostic|ArchivedExplicitSlug|OrphanActiveBundle)IsZeroWrite'`.
  - Broader package checks: `go test -count=1 ./internal/state`, `go test -count=1 ./cmd`, `go test -count=1 ./internal/tmpl`.
  - Final repo checks: `go test -count=1 ./...` and `go build ./...`.

## Alternatives Considered
- Approach A: Minimal freshness DAG correction plus docs/tests. Remove summary/task captured timestamps from the normal per-task freshness baseline, preserve fail-closed unreadable-artifact behavior, add #32 zero-write tests, and clarify #29 wording. Tradeoff: #30/#34 remain deferred enhancements.
- Approach B: Broaden repair/done to enforce tracked archived runtime evidence checks and orphan bundle diagnostics now. Tradeoff: higher blast radius and not required by the confirmed issue evidence; risks expanding beyond the user's final priority.
- Approach C: Change `validate --change` to read archived bundles. Tradeoff: violates active-only validate contract and mixes active runtime readiness with frozen archive audit.
- Selected: Approach A, matching the user's final triage. It fixes the only confirmed core bug (#28), clarifies #29 without changing semantics, and locks #32/#34 read-only boundaries without implementing deferred enhancements.

## Unknowns
- Resolved: Is #28 caused by summary self-staleness? -> Yes. The RED test reproduced stale freshness when summary `CapturedAt` was newer than otherwise matching task evidence.
- Resolved: Does current HEAD generate archived gitignore negations for runtime directories? -> No. `internal/state/local_ignore.go` has only positive ignore patterns for runtime dirs.
- Resolved: Does current validate no-active path write stale review artifacts? -> No current write path was found; zero-write regression tests now guard the no-active, archived explicit, and orphan active-bundle paths.
- Remaining: None for this change.

## Assumptions
- The user's final issue disposition is scope authority for this governed change. Evidence: request text and `intent.md`.
- Current HEAD behavior is authoritative for closing #30/#32/#34 unless reporters provide version-specific reproduction details. Evidence: `gh issue view` for #28/#34, current source inspection, and local tests.
- Non-empty orphan bundles should remain operator-reviewed integrity findings, not auto-deleted in this change. Evidence: `cmd/repair.go` non-repairable finding behavior and existing repair tests.

## Canonical References
- `artifacts/changes/fix-execution-summary-freshness-and-clarify-validate-diagnos/intent.md`
- `internal/state/execution_summary.go`
- `internal/model/execution_summary.go`
- `internal/state/execution_summary_test.go`
- `cmd/validate.go`
- `cmd/common.go`
- `cmd/validate_readonly_test.go`
- `docs/commands.md`
- `docs/operator-guide.md`
- `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl`
- `internal/tmpl/templates/artifacts/assurance.md`
