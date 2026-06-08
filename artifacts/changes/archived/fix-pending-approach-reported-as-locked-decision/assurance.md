# Assurance

## Scope Summary
Issue #140: `slipway next --json` no longer reports a recommended-but-
unconfirmed decision as a locked decision. The locked-vs-pending split is now
driven by the lifecycle `G_plan` gate (not the decision-text placeholder
matcher): a parsed `decision.md` "Selected Approach"/"Selected Direction" is
reported under `skill_constraints.locked_decisions` only once `G_plan` is
approved, and under the new `skill_constraints.pending_decisions` field
otherwise. The `spec-compliance-review` generated surface was updated to enforce
Decision Fidelity only against `locked_decisions` and treat `pending_decisions`
as advisory. Delivered across `cmd/next.go`, `cmd/next_skill.go`,
`cmd/next_skill_view.go`, `cmd/next_handoff.go`, the spec-compliance-review
template, and tests.

## Verification Verdict
PASS at run_version 1. All required governance skills carry passing verification
for run_version 1; both review stages passed; goal-verification passed with
fresh 3-level evidence; closeout re-ran the full suite green.
- `go build ./...` exit 0; `go vet ./...` exit 0.
- `go test ./...` green (24 packages) — see Evidence Index.
- Confirmed-path e2e: `TestSkillConstraintsLockedDecisionsAfterPlanLock`
  drives real `next --json --diagnostics` output for a governed change with
  G_plan approved; `skill_constraints` carries `locked_decisions` and not
  `pending_decisions`.

## Evidence Index
All under `artifacts/changes/fix-pending-approach-reported-as-locked-decision/`:
- `verification/intake-clarification.yaml` — pass
- `verification/research-orchestration.yaml` — pass
- `verification/plan-audit.yaml` — pass (8D)
- `verification/wave-plan.yaml` + `verification/execution-summary.yaml` (run_summary_version 1)
- runtime task evidence t-01..t-04 (all pass, run_summary_version 1)
- `verification/wave-orchestration.yaml` — pass
- `verification/spec-compliance-review.yaml` — pass (layer:R0, scope_contract, negative_path)
- `verification/code-quality-review.yaml` — pass (layer:IR1)
- `verification/goal-verification.yaml` — pass (AC-1..AC-7, scope_contract:pass)
- `verification/final-closeout.yaml` — this closeout

## Requirement Coverage
- REQ-001 (pending excluded from locked_decisions) → t-01; tests: not_planLocked
  routing + pending e2e.
- REQ-002 (surfaced under pending_decisions) → t-01; tests: pending e2e.
- REQ-003 (locked after G_plan approved) → t-01; tests: locked routing unit +
  TestPlanLockedFromGates + TestSkillConstraintsLockedDecisionsAfterPlanLock.
- REQ-004 (gate-derived, parser unchanged) → t-01; tests: TestPlanLockedFromGates;
  parser package diff empty.
- REQ-005 (preserved in clone) → t-02; tests: handoff e2e.
- REQ-006 (spec-compliance-review advisory) → t-03; tests:
  TestSpecComplianceReviewTreatsPendingDecisionsAsAdvisory.

## Residual Risks and Exceptions
- External JSON contract change (`next --json` `skill_constraints`): additive
  `pending_decisions` (omitempty) + tightened `locked_decisions` semantics. This
  is the intended fix for #140; reviewed as a contract change at S3 (the only
  in-repo consumer — the spec-compliance-review template + cloneSkillConstraints
  — was updated consistently; generated `.claude`/`.codex` copies are gitignored
  and regenerate via toolgen).
- The previous closeout draft incorrectly described the S4_VERIFY done-ready
  surface as carrying `next_skill.skill_constraints`; that is not true once
  `next_skill` is null. The repaired evidence uses
  `TestSkillConstraintsLockedDecisionsAfterPlanLock`, which exercises a real
  `next --json --diagnostics` response at an executable state where G_plan is
  approved and `next_skill.skill_constraints` is present.
- No accepted spec exceptions.

## Rollback Readiness
Additive, reversible change: an `omitempty` field, a single conditional split,
gate-state threading via an unexported view field, and a template-text update.
Rollback = revert the change (no data migration, no irreversible op). Rollback
verification: `go test ./...` on the reverted tree.

## Archive Decision
Ready to archive on `done`. Active-change freshness/readiness was proven by a
fresh `slipway validate --json` on this worktree at run_version 1 before `done`
(blockers empty, gates G_plan/G_scope approved, G_ship satisfied at closeout).
This attestation is for the ACTIVE bundle; once archived the bundle is a frozen
record and is not described as revalidated through the active validate gate.
