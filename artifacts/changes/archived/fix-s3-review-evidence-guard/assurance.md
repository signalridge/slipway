# Assurance

## Scope Summary

This change fixes issue #394 by failing closed at the `slipway evidence skill`
write boundary for passing selected S3 review evidence that does not include
exactly one valid `context_origin:stage=review=<handle>` reference. The scope
also updates generated/public guidance so selected-review pass commands include
the review context-origin handle, a `*-notes.md` notes file, and degraded
fallback as an additional structured reference rather than as a replacement for
the review handle.

The implementation is intentionally limited to `evidence skill` record-time
validation, the reusable context-origin parser helper, focused command/model
tests, capability/next/template/toolgen surfaces, and localized command docs.
It does not redesign S3 review selection, subagent dispatch, or task/wave proof
Markdown handling.

## Verification Verdict

Pass. The implementation, generated surfaces, review evidence, and terminal
verification commands all support the acceptance criteria. Passing selected S3
review evidence now rejects missing, malformed, conflicting duplicate, and
identical duplicate review context-origin references before reviewer YAML is
written. Valid selected-review pass evidence still records successfully, and
fail verdicts remain recordable without a review context-origin handle.

## Evidence Index

- `artifacts/changes/fix-s3-review-evidence-guard/verification/intake-clarification.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/research-orchestration.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/plan-audit.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/wave-orchestration.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/final-focused-verification.md`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/spec-compliance-review.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/code-quality-review.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/independent-review.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/security-review.yaml`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-coverage-gate.txt`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-go-vet.txt`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-testlint.txt`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-golangci-lint.txt`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-diff-check.txt`
- `artifacts/changes/fix-s3-review-evidence-guard/verification/logs/ship-manifest-check.txt`

## Requirement Coverage

- REQ-001: Covered by the pre-persistence candidate validation in
  `cmd/evidence.go`, exact-one review handle parsing in
  `internal/model/context_attestation.go`, and negative command tests that
  assert no reviewer YAML is written for invalid selected-review pass evidence.
- REQ-002: Covered by the guard's pass-only and selected-review-only scope and
  by the fail-verdict test that records selected-review fail evidence without a
  review context-origin handle.
- REQ-003: Covered by the evidence command partial, review skill templates,
  capability remediation, `next` confirmation text, surface manifest, and
  English/Chinese/Japanese command docs.
- REQ-004: Covered by focused command tests, parser tests, generated-surface
  tests, capability tests, and the affected package suite.

## Residual Risks and Exceptions

- `same_context_degraded` was used for S3 peer review and ship verification
  because the current host did not provide an explicitly user-requested
  subagent delegation flow. The degraded mode is explicitly recorded in review
  evidence and does not replace distinct review context-origin handles.
- Existing invalid selected-review YAML from before this change is not migrated.
  The existing readiness/ship gate still rejects such records as defense in
  depth; new invalid pass submissions are blocked before persistence.
- No guardrail domain was set for this change. The security review found no
  auth/authz, secrets, injection, SSRF, XSS, dependency, or infrastructure
  security impact.

## Rollback Readiness

Rollback is a normal code revert of the command guard, parser helper, tests, and
surface/docs updates. No data migration or state migration is required. After a
rollback, rerun the same command-boundary, model, template/toolgen/capability,
full-suite, static-check, and coverage-gate commands listed in the evidence
index before shipping the rollback.

## Archive Decision

Ready to archive after `ship-verification` passes and active `slipway validate`
is captured immediately before `done`. The active validation proof belongs to
the live governed worktree and will be cited by ship-verification; archived
bundles are retained records and are not revalidated through the active validate
gate after archival.
