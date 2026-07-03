# Assurance

## Scope Summary
Delivered issue #371 by adding structured plan-dimension attestations without
changing the `VerificationRecord` schema. The implementation adds the
`dim:<name>=<verdict>:<evidence-ref>` reference-token parser, S1 plan-audit
gate enforcement with the past-S1 no-rewalk guard, S3 selected
`spec-compliance-review` enforcement, CLI early rejection for incomplete passing
dimension evidence, deterministic unknown prose `REQ-*` validation, reason and
recovery mappings, and generated skill template instructions plus the
plan-audit consistency sidecar.

S3 review repairs are included in this closeout: the deterministic prose scanner
is case-sensitive so lowercase API prose such as `req-timeout` is not treated as
a requirement reference, S3 plan-dimension recovery now points to the owning
`spec-compliance-review` skill rather than S1 `plan-audit`, and placeholder
evidence detection rejects explicit placeholder tokens without rejecting valid
path segments named `todo` or `placeholder`. A later review-driven repair also
closes the directory-evidence gap: `decision_soundness` rejects exact
`artifacts` / `artifacts/` self-confirmation, and all attestation evidence must
resolve to a regular file rather than a broad directory.

## Verification Verdict
PASS. Focused package tests, full repo tests, build, vet, diff hygiene, and
golangci-lint all passed in the bound worktree after the S3 review repairs.

## Evidence Index
- `go test ./internal/model ./internal/engine/progression ./cmd ./internal/tmpl ./internal/toolgen`
- `go test ./internal/model -run 'TestPlanDimensionAttestationsFromVerification|TestPlanDimensionRecoveryUsesOwningSkillFromBlockerDetail'`
- `go test ./internal/engine/progression -run 'TestEvaluateReviewAuthorityRequiresSpecCompliancePlanDimensionAttestations|TestValidateTasksChecklist_RejectsUnknownRequirementRefsInArtifactProse|TestValidateTasksChecklist_IgnoresLowercaseReqHyphenWordsInArtifactProse|TestValidateTasksChecklist_DoesNotDuplicateUnknownCoversAsProseConsistency'`
- `go test ./...`
- `go build ./...`
- `go vet ./...`
- `gofmt -s -l ...`
- `git diff --check`
- `golangci-lint run`
- `go run . validate`
- Task evidence: `verification/t-01-result.json` through `verification/t-07-result.json`
- Wave evidence: `verification/wave-orchestration.yaml`
- Execution summary: `verification/execution-summary.yaml`

## Requirement Coverage
- REQ-001: Covered by `internal/model/plan_dimension_attestation.go`,
  `internal/model/plan_dimension_attestation_test.go`, reason-code contract
  tests, and `go test ./internal/model`.
- REQ-002: Covered by `EvaluatePlanGate` changes and
  `TestEvaluatePlanGate_RequiresPlanDimensionAttestationsAtS1Only`.
- REQ-003: Covered by `EvaluateReviewAuthority` changes and
  `TestEvaluateReviewAuthorityRequiresSpecCompliancePlanDimensionAttestations`.
- REQ-004: Covered by `cmd/evidence.go` validation and evidence skill tests for
  missing passing plan-audit/spec-compliance dimensions and failed-dimension
  evidence.
- REQ-005: Covered by new reason-code definitions, owner-aware S3 recovery
  mappings, `reason_code_contract_test.go`, `recovery_test.go`, and the review
  authority regression that asserts S3 recovery does not point to `plan-audit`.
- REQ-006: Covered by `validation.go` unknown prose requirement scanning and
  validation tests that reject uppercase unknown prose refs, ignore lowercase
  `req-*` API prose, and avoid duplicating unknown `covers`.
- REQ-007: Covered by plan-audit/spec-compliance-review template updates,
  `references/consistency-audit.md`, and toolgen inventory verification.
- REQ-008: Covered by the focused and repo-level verification commands listed
  above.

## Residual Risks and Exceptions
The implementation intentionally raises structural attestation floors, not
semantic truth guarantees. The engine now verifies that dimension tokens are
parseable, point at a resolvable regular file, and keep `decision_soundness`
outside `artifacts`; it still cannot prove that the cited file is a strong
counterexample or that the reviewer made the right judgement.

Attestation evidence remains filesystem-coupled by design. If a cited file is
renamed or removed during review, readiness fails closed with unresolved
evidence until the owning audit/review evidence is refreshed.

Light preset keeps S1 dimension-attestation gaps advisory to preserve the
approved preset semantics. The deterministic unknown uppercase `REQ-*` prose
check remains a hard artifact-consistency blocker across presets because it is a
declared-reference validity check, not a broader semantic-quality inference.

S3 review evidence in this run uses `same_context_degraded` fallback because no
host subagent delegation authorization was available in this session. The
fallback is recorded explicitly on each S3 review evidence record.

## Rollback Readiness
Rollback is a normal code revert: remove the model parser, gate/CLI/validation
call sites, reason/recovery entries, tests, and template/golden additions. No
schema migration or persisted evidence migration was introduced.

## Archive Decision
Not archived yet. Active validation and terminal ship-verification are captured
in this worktree before `done`; archiving is appropriate only after `slipway run`
reports `done_ready` and before the explicit `slipway done` finalization step.
