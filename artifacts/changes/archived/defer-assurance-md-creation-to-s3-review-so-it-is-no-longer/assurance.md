# Assurance

## Scope Summary
Three coupled changes, the latter two discovered while dogfooding the first and
bundled in at the user's request:

1. **Defer `assurance.md` creation to S3_REVIEW (issue #141).** `assurance.md` is
   added to `deferredToSkillAuthoring` (no early engine scaffold); its existence is
   owned solely by the assurance contract gate at S3_REVIEW and later, including
   DONE and explicit unknown lifecycle states. The generic
   pre-S3 existence gates (`GovernedBundleBlockers` and the artifact-readiness
   evaluator) skip it via a shared `existenceOwnedByDedicatedGate` predicate, so a
   deferred (absent) `assurance.md` no longer strands a change at S1/S2 — the gap
   the issue's literal proposal would have left open. Public surfaces (instructions
   guidance, the `slipway preset` re-scaffold comment, the plan-audit and
   final-closeout host-skill templates, `docs/workflow.md`) were aligned, and the
   obsolete plan-audit-digest exclusion comment removed.
2. **`slipway repair` / doctor bundle-consistency alignment.** `DiagnoseBundleConsistency`
   stays silent on a deferred (absent) `assurance.md` before S3_REVIEW — that is the
   expected deferred state, not a partial-write inconsistency — while keeping the
   S3_REVIEW/S4_VERIFY/done "missing assurance" consistency error.
3. **Out-of-scope drift recovery (rescope reachability + accurate guidance).**
   `slipway pivot --rescope` is now reachable from S2_EXECUTE/S3_REVIEW/S4_VERIFY
   (it resets to S0_INTAKE regardless of the starting state), so a review-time
   out-of-scope edit — which reopens to S3_REVIEW for stale review evidence — can
   reach the documented recovery; it stays rejected before execution
   (S0_INTAKE/S1_PLAN) with `rescope_state_invalid`. The `scope_contract_drift`
   guidance (CLI remediation + `scope_contract_recovery_guidance` diagnostic) was
   rewritten to lead with the non-destructive amend-`tasks.md`-`target_files` path
   and to describe `pivot --rescope` honestly as a full re-plan that resets to
   S0_INTAKE; `docs/commands.md` was aligned.

Out of scope and untouched: the #47 scaffold-placeholder floor (kept as a
backstop), rescope's S0_INTAKE-reset semantics (only its reachable states
broaden), and the timing of any other artifact.

## Verification Verdict
PASS after post-review repair. Current worktree evidence: `gofmt -l internal cmd`
clean; `git diff --check` clean; `go build ./...`, `go vet ./...`, and `go test
./... -count=1` all pass. Targeted regressions also pass for:
`TestTraceabilityMissingOrEmptyAssuranceIsStageAware`,
`TestTraceabilityCoherenceHealthIsStageAware`,
`TestAssuranceContractBlockers_MissingFile`, and
`TestHealthCommandDoctorTracksAssuranceBlockingState`.

The independent review found two blockers and both are repaired:
1. Missing or empty `assurance.md` now fails closed at DONE and explicit unknown
   lifecycle states, while preserving standalone traceability scans that omit
   lifecycle context.
2. The archived task scope and assurance record now reflect the current worktree
   shape: 30 non-artifact source/doc/test/template files are covered by
   `target_files`; the archived evidence files are updated separately.

## Evidence Index
- verification/intake-clarification.yaml — intake re-confirmed after scope expansion
- verification/plan-audit.yaml — plan audit (REQ-001..006 covered; tasks valid)
- verification/wave-orchestration.yaml — S2 execution (run_version 1, 9/9 tasks pass)
- verification/execution-summary.yaml — overall_verdict pass, run_summary_version 1
- verification/spec-compliance-review.yaml — Stage 1, layer:R0=pass, scope_contract:pass
- verification/code-quality-review.yaml — Stage 2, layer:IR1=pass
- verification/goal-verification.yaml — acceptance signals met, run_version 1
- post-review repair verification — `go test ./... -count=1`, `go build ./...`,
  `go vet ./...`, `gofmt -l internal cmd`, and `git diff --check` pass at the
  current worktree state
- scope_contract: target_files synchronized for all 30 non-artifact changed
  source/doc/test/template files in the current worktree; archived governance
  artifact files are updated as evidence, not task implementation targets

## Requirement Coverage
- REQ-001 (defer creation): `manager.go` `deferredToSkillAuthoring` += assurance.md — `TestScaffoldGovernedBundleDefersAssurance` (standard/strict/light).
- REQ-002 (no pre-S3 block; repair silent): `existenceOwnedByDedicatedGate` skip in `validation.go` + `readiness.go`; `DiagnoseBundleConsistency` silent pre-S3 in `repair.go` — `TestGovernedBundleBlockers_DefersAssuranceToContractGate`, `TestCheckGovernedBundleReadyDoesNotRequireAssuranceArtifact`, `TestDiagnoseBundleConsistencyAssuranceDeferredPreReviewIsSilent`.
- REQ-003 (fail-closed S3+; repair S3+ error): `AssuranceContractBlockers` now enforces review/verify/done and explicit unknown lifecycle states; `repair.go` S3+ error branch; health/traceability reports missing/empty assurance as blocking once the lifecycle reaches review/verify/done — `TestGovernedBundleBlockers_DefersAssuranceToContractGate` (S3 missing), `TestDiagnoseBundleConsistencyAssuranceMissingErrorInReview`, `TestAssuranceContractBlockers_MissingFile`, `TestTraceabilityMissingOrEmptyAssuranceIsStageAware`, `TestTraceabilityCoherenceHealthIsStageAware`, `TestHealthCommandDoctorTracksAssuranceBlockingState`.
- REQ-004 (deferred-authoring surfaces): instructions guidance, preset comment, plan-audit + final-closeout templates, `docs/workflow.md` — `TestInstructionsGuidanceMatchesScaffoldOwnership`.
- REQ-005 (digest exclusion prose removed): `addPlanningArtifactInputs` in `evidence_digests.go` (comment only; assurance.md remains a final-closeout digest input) — covered by the progression evidence-digests tests.
- REQ-006 (rescope reachable + accurate guidance): `gate.go` + `pivot_validation.go` (reachable S2/S3/S4, rejected before execution); `recovery.go` + `readiness.go` guidance; `docs/commands.md` — `TestEvaluateGPivot`, `TestValidatePivotPreconditionsAllowsRescopeFromS2ThroughS4`, `TestValidatePivotPreconditionsRejectsRescopeBeforeExecution`, `TestPivotRescopeReachableFromS3Review`, `TestValidateAndNextGuideS3ScopeContractDriftToRecoveryPath`.

## Residual Risks and Exceptions
- This change was implemented before its formal S2 evidence capture (a dogfooding
  bootstrap): the deferral fix is itself required to clear the pre-S3 assurance.md
  block that would otherwise strand a standard change at S1. The recorded S2
  evidence (run_version 1) reflects the actual, test-green implementation in this
  worktree.
- Scope grew from the single assurance-deferral objective to three coupled
  governed-surface fixes (user-approved). The two additions are narrow and
  fail-closed; flag the scope growth in the PR body.
- The `target_files` gate covers all task kinds (intentional, matches the hard
  tasks-checklist gate); flag in the PR body, not a defect here.
- No backward-compatibility shim for the retired early-scaffold behavior or the
  former S2-only rescope restriction, by design.
- Active `slipway validate --json --change ...` is intentionally unavailable for
  this slug because the change is archived as DONE; current evidence is the
  repaired archived bundle plus the fresh build/vet/test/diff checks above.

## Rollback Readiness
Low risk, fully reversible by `git revert` of the worktree branch — the change is
engine/comment/doc/test only, with no schema migration, data, or external-contract
surface. Reverting restores the prior early-scaffold behavior, the prior repair
warning, and the S2-only rescope restriction; no persisted state depends on any of
the three changes. Each wave is independently green; no partial/half-states exist.

## Archive Decision
Ready to finalize. All acceptance signals are met with fresh evidence at this
state, the governed bundle has been synchronized with the current worktree shape,
and the review blockers have been repaired. The change is already archived DONE,
so no active lifecycle transition remains.
