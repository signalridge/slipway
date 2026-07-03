# Intent

## Summary
Implement issue #371: extend plan-audit audit scope with structured semantic quality and decision soundness attestations
## Complexity Assessment
complex
Rationale: this change touches the model-level evidence contract, progression
gates, CLI evidence validation, recovery/reason contracts, generated skill
templates, and tests across S1 and S3 lifecycle surfaces.

## Guardrail Domains
None detected.

## In Scope
- Add a structured `dim:` reference-token contract for plan dimension
  attestations without changing the `VerificationRecord` schema.
- Require `decision_soundness` and `consistency` passing attestations for
  standard/strict S1 `plan-audit` evidence while keeping light advisory.
- Require the same attestations for selected S3 `spec-compliance-review`
  evidence, owned by review context rather than S1 audit origin.
- Add `slipway evidence skill` early validation for passing `plan-audit` and
  selected S3 `spec-compliance-review` records.
- Add reason/recovery contract coverage for the new blocker codes.
- Update plan-audit and spec-compliance-review skill templates, including a
  plan-audit consistency sidecar, and refresh generated host surfaces.
- Add deterministic consistency validation only for unknown prose `REQ-*`
  references where the engine can prove the reference is invalid.
- Add focused unit/contract tests plus repo-level verification.

## Out of Scope
- No `VerificationRecord` schema expansion.
- No automatic migration, bypass, or force-close path for old evidence.
- No required `blast_radius` or `test_mapping` dimension in this change.
- No generic semantic parser for terminology drift, stale code references, or
  target-file existence checks that are not deterministically safe.

## Constraints
- Preserve fail-closed behavior for missing, malformed, conflicting, or failed
  required attestations.
- Avoid lifecycle re-walk: do not make already-past-S1 changes rerun S1
  `plan-audit`; S3 review ownership handles late-stage attestations.
- Keep `decision_soundness` primary evidence outside `artifacts/` to avoid
  circular self-confirmation.
- Prefer current worktree lifecycle output over remembered command semantics.

## Acceptance Signals
- Parser tests cover valid, duplicate, conflict, malformed, placeholder,
  traversal/absolute, unresolvable, and `artifacts/` soundness evidence cases.
- S1 plan gate tests prove old tokenless passing `plan-audit` now blocks in
  S1, required tokens pass, fail tokens block, and past-S1 changes are not
  retroactively blocked by S1 evidence.
- S3 review authority tests prove selected `spec-compliance-review` requires
  review-owned required `dim:` tokens.
- Evidence CLI tests prove passing `plan-audit` and selected
  `spec-compliance-review` evidence is rejected without required tokens.
- Reason/recovery contract tests cover every new code.
- Generated skill/template tests and `go test ./...`, `go build ./...`,
  `go vet ./...`, `git diff --check`, and `golangci-lint run` are attempted
  with results reported.

## Open Questions
None.

## Deferred Ideas
- Promote `blast_radius` and `test_mapping` to required dimensions after the
  initial `dim:` contract has operational signal.
- Add broader deterministic semantic consistency checks only after concrete
  low-false-positive rules are proven.

## Approved Summary
Confirmed by user on 2026-07-03: implement GitHub issue #371 by adding
structured `dim:` attestations for `decision_soundness` and `consistency`,
enforcing them in S1 plan-audit, S3 spec-compliance-review, evidence CLI
validation, reason/recovery contracts, generated skill surfaces, and tests.
The change explicitly excludes a `VerificationRecord` schema change, old
evidence migration, bypass/force-close behavior, and required
`blast_radius`/`test_mapping` dimensions.
