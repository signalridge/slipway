# Assurance

## Scope Summary

Delivered issue #155 scope: prose artifact input digests now use a material
view instead of raw bytes. The change is limited to
`internal/engine/progression/evidence_digests.go` and
`internal/engine/progression/evidence_digests_test.go`.

## Verification Verdict

Pass. Focused digest tests and the full Go test suite passed after the
implementation. `go run . validate --json` reports fresh evidence, approved
`G_scope` and `G_plan`, and `scope_contract.status=pass`.

## Evidence Index

- RED test command:
  `go test -count=1 ./internal/engine/progression -run 'TestPlanAuditInputDigestIgnoresScaffoldOnlyProseEdits|TestProseFileInputHashTreatsKnownDefaultsAsNonMaterial|TestEvaluateRequiredSkillsUsesContentDigestNotMTime|TestResearchOrchestrationInputDigestIncludesResearchArtifact'`
  failed before implementation.
- Focused verification:
  `go test -count=1 ./internal/engine/progression` passed.
- Full verification:
  `go test -count=1 ./...` passed.
- Runtime task evidence exists for `t-01`, `t-02`, and `t-03` with
  `run_summary_version=1`.
- Current validation proof: `go run . validate --json` reports fresh evidence
  and scope-contract pass.

## Requirement Coverage

- REQ-001 is covered by
  `TestPlanAuditInputDigestIgnoresScaffoldOnlyProseEdits` and
  `TestProseFileInputHashTreatsKnownDefaultsAsNonMaterial`.
- REQ-002 is covered by the authored-prose branch in
  `TestProseFileInputHashTreatsKnownDefaultsAsNonMaterial`, plus the existing
  `TestEvaluateRequiredSkillsUsesContentDigestNotMTime` and
  `TestResearchOrchestrationInputDigestIncludesResearchArtifact` stale evidence
  tests.

## Residual Risks and Exceptions

No accepted exceptions. The main residual risk is future over-broad known
defaults; the current implementation keeps the known-default table narrow and
defaults unknown non-empty prose to material.

## Rollback Readiness

Rollback is a normal git revert of the two changed source/test files. No data
migration, dependency update, external API change, or generated surface update
is involved.

## Archive Decision

Not archived yet. The requested endpoint is done-ready, so `slipway done` must
not be run in this workflow unless the user later asks for finalization. Before
any future `done`, capture fresh active `validate --json` readiness proof.
