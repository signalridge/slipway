# Assurance

## Scope Summary

This change resolves the three open Lattice-related Slipway feedback issues in
the Slipway CLI:

- #183: `slipway evidence skill --notes-file` now resolves workspace-relative
  notes files against the active change's authoritative workspace, including a
  bound worktree.
- #192: post-S2 attempts to refresh task evidence or `wave-orchestration`
  evidence now keep the same fail-closed wrong-state errors while naming the
  S3/S4 replacement evidence surfaces.
- #189: skill handoff hard stops now state that the operator must run the named
  governance skill and record evidence, while `--resume-response` explicitly
  remains valid only for active checkpoints.

No lifecycle state, model schema, persisted field, bypass path, or force-close
path was added.

## Verification Verdict

Verification is passing for the implemented scope. The current governed evidence
chain includes passing wave execution, spec-compliance review, and
code-quality review for run version 1. Fresh command verification in the current
worktree passed:

- `go test -count=1 ./...`
- `go vet ./...`
- `git diff --check`

## Evidence Index

- Runtime task evidence:
  `.git/slipway/runtime/changes/resolve-lattice-workflow-feedback-issues-183-189-192/evidence/tasks/`
- Wave orchestration evidence:
  `verification/wave-orchestration.yaml`
- Spec compliance review evidence:
  `verification/spec-compliance-review.yaml`
- Code quality review evidence:
  `verification/code-quality-review.yaml`
- Review notes:
  `verification/spec-compliance-review-notes.md`
  `verification/code-quality-review-notes.md`
- Lifecycle validation:
  `slipway validate --json` reported `scope_contract.status=pass` after review
  evidence was recorded, with only this assurance artifact missing at that time.

## Requirement Coverage

- REQ-001 is covered by `cmd/evidence.go` and
  `TestEvidenceSkillNotesFileUsesBoundWorktreeWorkspace`, which proves
  bound-worktree `artifacts/...` notes files are read from the bound workspace.
- REQ-002 is covered by `cmd/evidence.go`,
  `TestEvidenceTaskWrongStateInS3RoutesToReviewAndVerificationEvidence`,
  `TestEvidenceTaskWrongStateInS4RoutesToGoalVerificationAndFinalCloseout`,
  `TestEvidenceSkillWrongStateForWaveOrchestrationInS3RoutesToReviewAndVerificationEvidence`,
  and `TestEvidenceSkillWrongStateForReviewEvidenceInS4RoutesToVerificationEvidence`.
- REQ-003 is covered by `cmd/next.go`, `cmd/next_context_build.go`,
  `TestConfirmationRequirementDistinguishesHardStopFromCommandBoundary`,
  `TestNextHandoffViewUsesStructuredConfirmationRequirement`,
  `TestRunRejectsResumeResponseWithoutCheckpoint`, and
  `TestRunAutoAdvanceIntoReviewSurfacesSkillHandoff`.

## Residual Risks and Exceptions

No known residual code blockers remain. The change intentionally does not add a
new post-review execution evidence command; it documents the current accepted
replacement path through review and verification evidence while preserving the
existing fail-closed lifecycle boundary.

The wave plan had to be amended to include `cmd/lifecycle_commands_test.go`
after scope-contract drift detected that legitimate test assertion update. The
wave evidence was then refreshed against the amended plan.

## Rollback Readiness

Rollback is a normal git revert of this branch's code, test, and governed
artifact changes. No migration, schema update, generated state change, or
external system contract was introduced. Reverting restores the previous CLI
wording and notes-file behavior.

## Archive Decision

The change is ready to proceed through S4 verification and final closeout. Active
`validate --json` freshness/readiness proof must be captured again after this
assurance file is authored and before `slipway done` is ever run. Once the ship
gate reaches done-ready, the archive can record this governed bundle as the
completed delivery evidence for issues #183, #189, and #192.
