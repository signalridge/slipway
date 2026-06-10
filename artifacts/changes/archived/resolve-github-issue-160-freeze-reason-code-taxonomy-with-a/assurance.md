# Assurance

## Scope Summary

Delivered GitHub issue #160 and the follow-up independent-review repair. The
reason-code taxonomy is now an explicit canonical contract; unrecognized codes
fail closed to `unknown_reason_code` while preserving the raw producer token in
`detail`; and repo-local tests reject direct assertions against unstable
reason/error `Message` prose when stable fields are available.

The repair closes the review findings by adding existing bulk-done and workflow
producer tokens to the canonical taxonomy and recovery map, preserving reachable
status wave error codes, and wrapping non-reason-domain CLI `error_code` values
in a canonical `wave_execution_unavailable` reason instead of letting them
collapse to `unknown_reason_code`.

The follow-up review repair tightens producer closure: in-scope producer specs
assert the raw produced token is canonical before `ReasonCodeFromSpec`
normalization, so an unknown producer cannot collapse to `unknown_reason_code`
and still pass the canonical-map check. The old production
`humanizeReasonCode` fallback was removed; tests keep a test-only
`testHumanizeReasonCode` helper as the legacy fallback baseline for proving
canonical messages are authored prose.

Out of scope remains unchanged: issue #152, a full JSON error schema redesign,
broad reason-code renaming, new external lint dependencies, and unrelated
workflow repair.

## Verification Verdict

Execution run_summary_version 1 passed all four planned tasks, review evidence,
goal verification, and final-closeout closeout checks. Fresh post-repair
verification evidence shows:

- `go test -count=1 ./internal/model -run 'TestInScopeProducedBlockersResolveToCanonicalRecovery|TestCanonicalReasonCodeTaxonomySnapshot|TestNewReasonCodeMakesUnknownCodeExplicit|TestReasonCodeNormalizeMakesUnknownCodeExplicit|TestReasonAndErrorContractTestsDoNotTextMatchMessageProse|TestMessageProseAssertionLint|TestWaveReasonCodesCarryRemediation|TestRequiredSkillStaleCarriesRemediation'` passed.
- `go test -count=1 ./...` passed across all 25 packages.
- `go vet ./...` passed.
- `go build ./...` passed.
- `go test -count=1 -coverprofile=/tmp/slipway-issue160-cover.out ./cmd ./internal/model ./internal/engine/progression ./internal/engine/gate ./internal/state` passed; `go tool cover -func=/tmp/slipway-issue160-cover.out` reported 73.4% total statement coverage across the affected package set.
- `gofmt -l cmd internal` produced no output.
- `git --no-pager diff --check` passed.
- Target-file stub and placeholder scans found no TODO/FIXME/HACK/PLACEHOLDER/NotImplemented/not implemented or production stub; mechanical hits were ordinary guard/helper returns and error-path tuples.
- `/tmp/slipway-issue160-closeout validate --json` in S4_VERIFY reported `evidence_freshness=fresh`, `scope_contract.status=pass`, and `goal-verification=pass` before final-closeout evidence was recorded; remaining blockers at that point were only final-closeout and its required assurance attestation.

## Evidence Index

- `verification/intake-clarification.yaml`: intake scope and user confirmation.
- `requirements.md`: REQ-001 through REQ-003 contract.
- `tasks.md`: four-wave execution plan and current target-file scope, including `cmd/status_view_build.go`.
- `verification/plan-audit.yaml`: approved planning evidence.
- `verification/wave-plan.yaml`: materialized execution plan.
- `verification/wave-orchestration.yaml`: wave execution verdict and task-result notes.
- `verification/execution-summary.yaml`: run_summary_version 1, all tasks pass.
- `verification/spec-compliance-review.yaml`: R0 spec-compliance pass after the review repair.
- `verification/code-quality-review.yaml`: IR1 code-quality pass after the final repair pass.
- `verification/goal-verification.yaml`: S4 acceptance criteria pass with fresh tests, build, vet, coverage, scan, and scope-contract references.
- `verification/final-closeout.yaml`: final closeout evidence with `closeout:assurance_complete=pass` for the standard workflow preset.
- `.git/slipway/runtime/changes/resolve-github-issue-160-freeze-reason-code-taxonomy-with-a/evidence/tasks/t-01.json`
- `.git/slipway/runtime/changes/resolve-github-issue-160-freeze-reason-code-taxonomy-with-a/evidence/tasks/t-02.json`
- `.git/slipway/runtime/changes/resolve-github-issue-160-freeze-reason-code-taxonomy-with-a/evidence/tasks/t-03.json`
- `.git/slipway/runtime/changes/resolve-github-issue-160-freeze-reason-code-taxonomy-with-a/evidence/tasks/t-04.json`

## Requirement Coverage

- REQ-001 Freeze Canonical Reason Codes: covered by `TestCanonicalReasonCodeTaxonomySnapshot`, the explicit `canonicalReasonDefinitions` map, and the severity snapshot in `internal/model/reason_code_contract_test.go`.
- REQ-002 Unknown Codes Are Not Silently Humanized: covered by `TestNewReasonCodeMakesUnknownCodeExplicit`, `TestReasonCodeNormalizeMakesUnknownCodeExplicit`, `newUnknownReasonCode`, and the status bridge tests that preserve canonical reason codes while wrapping non-reason CLI error codes.
- REQ-002 producer closure: covered by `TestInScopeProducedBlockersResolveToCanonicalRecovery`, which verifies raw producer tokens are canonical before normalization, and by `TestDoneBulkFallbackReasonCodesAreCanonical`, which proves all bulk-done fallback producer tokens remain stable canonical `.code` values and stay recovery-routable.
- REQ-003 Tests Assert Stable Contracts Instead Of Message Prose: covered by `TestReasonAndErrorContractTestsDoNotTextMatchMessageProse`, the lint bypass/scope tests, and migrated assertions in `cmd`, `internal/engine/gate`, `internal/engine/progression`, `internal/model`, and `internal/state`.
- Issue #160 documented enum acceptance: covered by `docs/operator-guide.md`, which documents `code` as the stable machine enum, `message` as presentation prose, `unknown_reason_code` behavior, and the reason-code versus CLI `error_code` bridge boundary.

## Residual Risks and Exceptions

- The AST lint remains intentionally syntactic. It blocks direct known payload
  `Message` prose assertions and can be extended when new reason/error payload
  surfaces appear, but it is not a typed whole-repo linter.
- Whole-package coverage percentages are diagnostic. Behavioral confidence for
  this repair comes from the targeted bulk-done/status regressions plus the
  taxonomy/recovery contract tests and the full suite.
- No guardrail domain, dependency/toolchain manifest change, data migration,
  credential handling, or irreversible operation is involved.

## Rollback Readiness

Rollback is a standard code revert of this branch's changes to reason-code
normalization, canonical definitions, recovery mapping, status bridge handling,
tests, operator-guide documentation, and governed artifacts. No data migration,
external API deployment, credentials, or irreversible operation is involved.

## Archive Decision

This change is ready for final-closeout acceptance and then `done`. Active
`/tmp/slipway-issue160-closeout validate --json` proof was captured in
S4_VERIFY before `done`; it reported fresh evidence, passing scope contract, and
passing goal-verification, with only final-closeout and the standard-preset
assurance attestation pending before `verification/final-closeout.yaml` is
accepted. After final-closeout records `closeout:assurance_complete=pass`,
`slipway run` should surface done-ready and `slipway done --json` should archive
the governed bundle.
