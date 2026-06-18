# Assurance

## Scope Summary

This change delivers the selected Approach D for issue #258: an internal
proof-reuse edge/check shape in `internal/engine/progression`, with the first
production edge limited to the existing final-closeout reuse of
goal-verification proof. The public closeout evidence boundary remains stable:
`closeout:goal_verification_reuse=pass`,
`closeout:goal_verification_reuse_run_version=<run_version>`, and
`closeout_goal_verification_reuse_invalid`.

The delivered scope also includes fail-closed suite-result freshness checks,
guardrail-domain SAST digest requirements for suite-result reuse, generated
goal-verification/final-closeout guidance updates, and a narrow evidence CLI
recovery path for selected reviewers whose prior passing record used a retired
or malformed context-origin token.

## Verification Verdict

The active change is not done at the time this assurance was authored because
final-closeout evidence has not yet been recorded. The implementation and
selected S3 review evidence are ready for closeout:

- `go test ./... -count=1` passed in the bound worktree and is captured in
  `artifacts/changes/generalize-digest-proof-reuse/verification/full-suite-transcript.txt`.
- `verification/suite-result.yaml` records run summary version `1` and full
  suite digest `ac32b2f4747c7ff7713488d419b1600ed4cb3a82810b0d1463b441412e23bd03`.
- Fresh selected S3 evidence is recorded and stamped for
  `spec-compliance-review`, `code-quality-review`, `independent-review`, and
  `goal-verification`.
- Active `go run . validate --json` after S3 restamping reported
  `evidence_freshness=fresh`, `scope_contract.status=pass`, and all four
  selected review skills passing. The remaining blockers at that point were the
  expected missing `final-closeout` evidence and standard-preset closeout
  attestations.

## Evidence Index

- Requirements: `artifacts/changes/generalize-digest-proof-reuse/requirements.md`.
- Decision: `artifacts/changes/generalize-digest-proof-reuse/decision.md`.
- Tasks: `artifacts/changes/generalize-digest-proof-reuse/tasks.md`.
- Execution summary:
  `artifacts/changes/generalize-digest-proof-reuse/verification/execution-summary.yaml`.
- Full-suite transcript:
  `artifacts/changes/generalize-digest-proof-reuse/verification/full-suite-transcript.txt`.
- Suite-result keystone:
  `artifacts/changes/generalize-digest-proof-reuse/verification/suite-result.yaml`.
- Fresh S3 review notes:
  `spec-compliance-review-notes.md`, `code-quality-review-notes.md`,
  `independent-review-notes.md`, and `goal-verification-notes.md` under
  `artifacts/changes/generalize-digest-proof-reuse/verification/`.
- Fresh selected S3 verification records:
  `spec-compliance-review.yaml`, `code-quality-review.yaml`,
  `independent-review.yaml`, and `goal-verification.yaml` under
  `artifacts/changes/generalize-digest-proof-reuse/verification/`.
- Active lifecycle proof after S3 restamping: `go run . validate --json` and
  `go run . next --json --diagnostics` run from the bound worktree on
  2026-06-18.

## Requirement Coverage

- REQ-001 is covered by `proofReuseEdge`, `proofReuseDigestCheck`, and
  `proofReuseEdgeBlockers` in `internal/engine/progression/authority.go`, with
  tests covering a non-closeout source/consumer edge.
- REQ-002 is covered by the closeout compatibility wrapper preserving the
  existing public reuse references and unchanged
  `closeout_goal_verification_reuse_invalid` blocker.
- REQ-003 is covered by strict `verification/suite-result.yaml` loading,
  run-summary version matching, selected-review suite-result dependency, and
  guardrail-domain SAST digest enforcement in
  `internal/engine/progression/evidence_digests.go`.
- REQ-004 is covered by the generated skill-template updates that make
  goal-verification a selected S3 peer producing or refreshing
  `verification/suite-result.yaml`, and make final-closeout reuse depend on
  engine-validated freshness rather than an unsafe lifecycle advance.
- REQ-005 is covered by focused engine, command, and template tests, plus the
  fresh full-suite transcript recorded after the latest code and artifact
  changes.

## Residual Risks and Exceptions

No blocking product risk is accepted for this change. The selected reviewers
identified one non-blocking residual: the new evidence CLI recovery path fixes
retired or malformed selected-review context-origin records, but it does not
try to repair already well-formed yet colliding review handles. That broader
`cross_stage_context_not_distinct` recovery ergonomics is outside this scoped
issue and does not weaken the current proof-reuse gates.

This change has no active guardrail domain, so no SAST baseline token is
required for this specific change. The guardrail SAST digest requirement is
nevertheless covered by tests and helper fixture updates.

## Rollback Readiness

Rollback is a normal git revert of the implementation, tests, generated-template
updates, and governed artifacts in this branch. Because the public closeout
reuse references and blocker code are preserved, rollback does not require
migrating existing governed evidence records.

No irreversible data, schema, external API, credential, or deployment operation
is part of this change.

## Archive Decision

Archive readiness decision: proceed to fresh final-closeout review before done.

Active `go run . validate --json` proof was captured before `done` after S3
review evidence was refreshed. That active validation showed fresh lifecycle
evidence, a passing scope contract, and passing selected review skills, with
only the expected final-closeout and assurance-attestation blockers remaining.
Archived bundles are not treated as active validation inputs; this assurance is
for the still-active governed change and must be rechecked by final-closeout
before archival or done-ready status is claimed.
