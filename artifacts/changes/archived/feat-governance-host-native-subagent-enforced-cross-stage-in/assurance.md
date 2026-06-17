# Assurance

## Scope Summary

This change implements the review-set rescope for Slipway governance issue #240.
It keeps the chain-wide context-origin foundation and adds the selected S3 review
set:

- mandatory `spec-compliance-review`
- mandatory `code-quality-review`
- mandatory `independent-review`
- conditional `security-review` when the engine-derived security-review control
  is selected

The implementation adds the shared `context_origin:stage=review=<handle>` review
wire token, keeps review lattice participants keyed by recording skill name,
derives security-review selection from engine controls, threads the selected
review set through routing, requiredness, review authority, ship authority, and
closeout ordering, and promotes independent/security review to workflow-owned S3
host templates.

The scope remains inside the planned model, control, governance, progression,
capability/toolgen/template, command-surface, docs, codebase-map, and governed
artifact files. Current `go run . validate --json` reported
`scope_contract.status=pass`.

## Verification Verdict

S2 execution evidence reports `overall_verdict: pass` for tasks `t-00` through
`t-10` at `run_summary_version: 1`.

The current spec-compliance review verdict is PASS:

- `layer:R0=pass`
- `scope_contract:pass`
- `negative_path:pass`
- `context_origin:stage=review=spec-review-r2-20260617-codex-fresh`

The review confirmed bidirectional artifact-to-code alignment, no locked decision
violation, no untracked spec behavior, and concrete negative-path coverage for
the fail-closed review-set paths.

Current governance is not ship-ready yet. The active S3 selected review set still
requires code-quality-review, independent-review, and security-review evidence
after this spec-compliance review.

## Evidence Index

- `go run . next --json --diagnostics`: S3_REVIEW, selected reviewers are
  spec/code/independent/security, required R0 token present.
- `go run . instructions assurance --json`: assurance contract and required
  sections read before authoring this file.
- `go run . validate --json`: contracts valid, freshness fresh, scope contract
  pass, expected S3 blockers before review evidence.
- `artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/execution-summary.yaml`:
  `overall_verdict: pass`, tasks `t-00` through `t-10`, `run_summary_version: 1`.
- `artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/wave-orchestration.yaml`:
  passing wave-orchestration record with executor handles for refreshed parallel
  waves and focused integration checks.
- `artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/plan-audit.yaml`:
  passing plan-audit record with distinct `plan_origin` and `audit_origin`.
- `artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/spec-compliance-review-notes.md`:
  current R0 spec-compliance trace and verdict.
- Focused reviewer tests:
  `go test ./internal/model ./internal/engine/control ./internal/engine/skill ./internal/engine/progression ./internal/tmpl ./internal/toolgen ./cmd -run 'TestContextOrigin|TestReviewOrigin|TestDeriveControls_SecurityReview|TestEvaluateRequiredSkillsForChange_S3SecurityReviewSelection|TestResolveNextSkill_S3Review|TestReviewAuthoritySelectedPassingSkillsIgnoreUnselectedSecurityEvidenceOnDisk|TestCrossStageContextDistinctBlockers|TestShipCrossStageContextNoDoubleFire|TestPromotedReviewTemplatesEmitReviewContextOriginHandle|TestSpecComplianceReviewTemplateEmitsReviewContextOriginHandle|TestCodeQualityReviewTemplateEmitsReviewContextOriginHandle|TestRenderCatalogSkillUsesTypedTemplatesForProductionSkill|TestEvidenceSkillRecordsSelectedReviewPeerWithoutSpecPredecessor' -count=1`
  passed.
- `gofmt -s -l` on selected Go files passed with no output.
- `git diff --check` passed with no output.

## Requirement Coverage

| Requirement | Coverage summary | Current assurance |
| --- | --- | --- |
| REQ-001 | Pure context-origin grammar, `StageContextReview`, plan/audit tokens, executor handle set, collision helper, retired `review_origin`. | Covered by model implementation and `context_attestation_test.go`; active templates use `context_origin:stage=review`. |
| REQ-002 | Plan-audit author/auditor distinctness at S1. | Covered by `EvaluatePlanGate` and `advance_governed_test.go`. |
| REQ-003 | Variable concurrent S3 review set, mandatory trio plus optional security, unordered peers. | Covered by selected-review helper, `ResolveNextSkillWithReviewSelection`, required-skill evaluation, CLI surfaces, and tests. |
| REQ-004 | Engine-derived `ControlSecurityReview` selector. | Covered by control derivation, preset policy, governance actions/health/runtime surfaces, and selector tests. |
| REQ-005 | One selected-review-skill set feeds routing, requiredness, and authority surfaces. | Covered by `ReviewSkillSelectionFromControls`, required-skill filtering, readiness, review/ship authority, and command tests. |
| REQ-006 | Review lattice keyed by recording skill and sourced from `passingSkills`. | Covered by authority implementation and negative tests for same handle, missing handle, executor collision, absent record silence, and unselected security on disk. |
| REQ-007 | Ship authority and closeout ordering use the same selected review set. | Covered by `closeoutChainOrderBlockers`, ship lattice selected stages, and authority tests. |
| REQ-008 | Independent/security review are standalone workflow-owned S3 hosts. | Covered by governance registry, capability registry, toolgen descriptors, `.tmpl` host templates, and template/toolgen tests. |
| REQ-009 | Public CLI/generated surfaces expose the variable review set coherently. | Covered by `next`, `status`, `validate`, `evidence`, `review`, stale recovery, fixtures, and current CLI output. |
| REQ-010 | Reason/recovery/docs/verification prove fail-closed design. | Covered by canonical reason/recovery files, docs, template tests, command tests, focused reviewer tests, and S2 execution evidence. |

## Residual Risks and Exceptions

- Remaining S3 reviews are not complete. Code-quality-review, independent-review,
  and security-review must still run with distinct
  `context_origin:stage=review=<handle>` values before the review set can pass.
- The distinct-context guarantee remains structural. Handles are host-emitted
  strings, not cryptographic proof of independent execution; this residual is
  documented in `docs/design.md` and `docs/workflow.md`.
- Non-emitted comments in `skill_resolution.go` and `toolgen_test.go` still use
  old "peer pair" wording. Behavior, tests, CLI output, and generated S3 review
  hosts use the selected review set; this wording should be cleaned later to
  prevent reader confusion.
- The static support catalog for security-review still carries command/review
  path-trigger hints. The S3 workflow-owned host is generated from the `.tmpl`
  surface and the engine selector is `ControlSecurityReview`, so this is not
  treated as S3 selector evidence.

## Rollback Readiness

Rollback is mechanical but broad: remove `ControlSecurityReview`, the selected
review-set routing and requiredness changes, the selected-review lattice feeder,
selected-set ship/closeout ordering, promoted independent/security workflow host
templates, updated public surfaces, docs, fixtures, and tests. Because review
handles ride `VerificationRecord.References`, no data schema migration is needed.

Rollback must preserve or deliberately revert the earlier context-origin
foundation according to the chosen rollback scope. A narrow rollback can remove
the review-set rescope while leaving the chain-wide context-origin foundation in
place; a full #240 rollback would also revert the context-origin lattice and
plan/audit origin enforcement.

## Archive Decision

Not archive-ready yet. This assurance is authored at S3_REVIEW to satisfy the
assurance artifact contract and document the current evidence, but the change
must still complete the remaining selected S3 reviews and subsequent S4
verification before `done`.

No active pre-`done` `validate --json` readiness proof has been captured for
archive readiness yet. The latest active validation before this review showed
fresh evidence and `scope_contract.status=pass`, but it was correctly blocked by
missing assurance and missing selected review evidence.
