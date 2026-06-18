# Structure

Re-authored for change `generalize-digest-proof-reuse` (#258).

## Current Change Focus: Digest-Keyed Proof Reuse

The active change is structurally bounded to the progression engine's proof
freshness path and the generated governance host templates:

- `internal/engine/progression/authority.go` owns ship authority and the current
  closeout -> goal-verification reuse validation. It is the target for the
  internal proof-reuse edge/check extraction and for preserving the existing
  closeout compatibility wrapper.
- `internal/engine/progression/evidence_digests.go` owns digest input assembly
  for goal-verification and final-closeout. It is only in scope for renaming or
  sharing helpers when the helper is genuinely proof-reuse-neutral.
- `internal/engine/progression/authority_test.go` and
  `internal/engine/progression/evidence_digests_test.go` are the focused tests
  for valid reuse, stale run versions, changed content, missing suite-result
  proof, and missing guardrail SAST digest behavior.
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` and
  `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl` own the
  agent-facing proof production/reuse guidance. `internal/tmpl/templates_test.go`
  pins the generated wording and token contracts.

The change should not introduce a new top-level package, public evidence struct,
or broad registry. The intended shape is a small internal validator in
`internal/engine/progression` with only the existing closeout reuse edge enabled
in production.

Re-authored for change
`feat-governance-host-native-subagent-enforced-cross-stage-in` (#240).

Baseline: #239 (engine-consumed reviewer-independence + context-origin/`review_origin`
attestation, closeout chain-ordering, and the wave dispatch/executor gates) has
SHIPPED (commit 2d2adac, 0.26.0); this change builds on that baseline and retires
the per-review-pair `review_origin` grammar in favor of the chain-wide
`context_origin:stage=` lattice. This refresh supersedes the stale S3 pair
plan: the current structure uses one selected review set with a mandatory
spec/code/independent trio and optional security from governance controls.

- `internal/model/` (pure layer; no cmd/tmpl/toolgen imports)
  - `context_attestation.go` (t-01, DONE): the unified attestation grammar and
    lattice. `ContextOriginReferencePrefix = "context_origin:stage="` (:38),
    `PlanOriginReferencePrefix = "plan_origin:"` (:43),
    `AuditOriginReferencePrefix = "audit_origin:"` (:48); eight `StageContext*`
    constants (:22-30: executor, plan_origin, audit_origin, review, spec, code,
    goal, closeout). Review hosts all emit `stage=review`, while the authority
    feeder keys lattice participants by selected review skill name. Types
    `ContextOriginHandle{Stage,Handle}` (:53) and
    `ContextParticipant{Handle,HandleSet}` (:181). Parsers
    `ContextOriginHandlesFromVerification` (:65, fail-closed/idempotent, first-`=`
    split via `parseContextOriginReference` :86),
    `PlanOriginHandleFromVerification` (:111) / `AuditOriginHandleFromVerification`
    (:120) via `singleStageHandleFromVerification` (:124) and
    `parseStageOriginReference` (:145), `ExecutorParticipantHandleSetFromVerification`
    (:164, drops blank collapse, never nil). Lattice helpers
    `CrossStageContextCollisions` (:194) + `participantsCollide` (:218). Sibling
    `DegradedDispatchJustificationsFromVerification` (:248) +
    `WaveDegradedJustificationReferencePrefix` (:240) are pre-existing, unrelated.
    `review_origin` symbols are GONE (no compat shim).
  - `context_attestation_test.go`: exercises the whole grammar
    (`TestContextOriginHandlesFromVerification`, `TestPlanOriginHandleFromVerification`,
    `TestAuditOriginHandleFromVerification`,
    `TestExecutorParticipantHandleSetFromVerification`,
    `TestCrossStageContextCollisions`,
    `TestDegradedDispatchJustificationsFromVerification`,
    `TestContextAttestationPrefixConstsArePinned` pinning the four prefixes + eight
    stage constants). The lone `review_origin` hit is a negative fixture (:64,
    "wrong prefix" expecting ok=false).
  - `verification.go`: `VerificationRecord`; `References []string` is the SOLE
    host-controlled inbound channel (Timestamp + RunVersion engine-owned).
  - `reason_code.go` (t-02, DONE): `canonicalReasonDefinitions` registry (:40).
    NEW `context_origin_handle_invalid` (:501), `cross_stage_context_not_distinct`
    (:505), `plan_audit_origin_invalid` (:509) — all `ReasonSeverityError`.
    `review_origin_handle_invalid` is removed (renamed to
    `context_origin_handle_invalid`). `NewReasonCode` silently downgrades
    unregistered codes.
  - `recovery.go` (t-02, DONE): `blockerRemediations` map (:103). The three new
    codes at :517/:522/:530, all `RecoveryClassRerunSkill` + `slipway run`,
    routing to a fresh native subagent re-run of the owning stage.
  - `reason_code_contract_test.go`: `TestCanonicalReasonCodeTaxonomySnapshot` +
    `canonicalReasonCodeSnapshot` (:41) / `canonicalReasonSeveritySnapshot` (:218)
    frozen lists (new codes at :70/:71/:128, all frozen error);
    `TestReviewOriginHandleVocabularyRetired` (:274) asserts the renamed code is
    fully retired.
  - `recovery_test.go`: the fourth contract surface. `inScopeProducedRecoverySpecs`
    (:131) lists all three new codes (:158-160) and selected-review collision
    details now use skill-name endpoints;
    `TestInScopeProducedBlockersResolveToCanonicalRecovery` and
    `TestRemediationTableEntriesAreComplete` (:74) couple recovery.go to
    reason_code.go.
  - `wave_execution.go`: `ExecutorAgentHandlesFromVerification` (:336) reused
    unchanged as the executor stage's upstream source.
- `internal/engine/progression/`
  - `authority.go` (t-03, DONE): review + ship verdict gates consuming the
    selected-set lattice. `evaluateReviewAuthorityWithPolicy` (:79) derives the
    security selector, computes selected reviewers, and appends
    `crossStageContextDistinctBlockers` (:657) over
    `crossStageContextReviewStagesForSelectedSkills` (:691).
    `buildShipAuthorityFromReadiness` (:140) re-loads via
    `mergeContextHandleRecords` (:624), adds goal+closeout
    (`crossStageContextShipStagesForSelectedSkills`, :699), and dual-surfaces
    blockers into `VerifySkillBlockers` and `EvaluateGShip`.
    `crossStageContextParticipants` (:525) builds participants (executor set,
    audit_origin, selected review skills from `stage=review`, goal/closeout single
    handles). Owned-edge sets are computed by
    `crossStageContextOwnedReviewStagesForSelectedSkills` (:487) and
    `crossStageContextOwnedShipStages` (:505), yielding variable counts from the
    mandatory trio to the selected-security quartet. Closeout facets
    `closeoutAssuranceAttestationBlockers` (:262),
    `closeoutReviewerIndependenceBlockers` (:379, REQ-001 presence),
    `closeoutChainOrderBlockers` (:430, always-on vs the opt-in reuse token but
    preset-gated, `closeout_chain_order_invalid`, comparing every selected
    reviewer before goal and final closeout after goal);
    `closeoutGoalVerificationReuseBlockers` (:285) keeps only the opt-in reuse
    facet after the ordering halves moved into the always-on chain-order gate.
  - `advance_governed.go` (t-04, DONE): `EvaluatePlanGate` (:958), now preset-aware
    (`presetPolicy governance.PresetPolicy` param). `planAuditOriginHandleBlockers`
    (:996) owns the local `plan_origin != audit_origin` self-audit edge,
    `planAuditOriginInvalidBlocker` (:1021) builds `plan_audit_origin_invalid`;
    `enforced` is `EffectivePreset != light` (:970, advisory on light). Call wiring
    `CheckGateWithIteration` (:582) -> `EvaluatePlanGate` (:588, preset at :587).
    Single-tuple `ResolveNextSkill` caller at :201.
  - `readiness.go`: read/readiness path. `evaluateGateReadiness` calls
    `EvaluatePlanGate` (:488, preset at :487), symmetric with the advance path.
  - `skill_resolution.go`: `ResolveNextSkill` (:23) returns a skill slice.
    `S3_REVIEW` returns `skill.SelectedReviewSkills(reviewSelection)` (:39-42),
    so selected review peers dispatch concurrently and none precedes another.
    `ReviewSkillSelectionFromControls` (:72) turns `ControlSecurityReview` into
    the optional security selector. S0/S1/S2/S4 stay effectively single-skill.
    `PrimaryNextSkill` (:51) is only a compatibility projection.
  - `constants.go`: skill-name constants for review, verify, closeout, and support
    skills.
  - `stale_evidence_recovery.go`: recovery surfaces use the selected review set
    while retaining a conventional primary authority skill only where ordering
    needs exactly one representative.
  - `authority_test.go`, `advance_governed_test.go`: gate coverage.
    `TestShipCrossStageContextNoDoubleFire` (`authority_test.go:568`) proves review
    edges do not re-fire at ship; `TestEvaluatePlanGate_PlanAuditSelfAuditEdge`
    (`advance_governed_test.go:40`) proves distinct->pass / equal->blocked /
    missing-audit_origin->blocked / light->advisory.
  - `skill_resolution_test.go`, `stale_evidence_recovery_test.go`,
    `evidence_test.go`, and command tests cover selected review routing, missing
    selected review blockers, selected-security behavior, and primary-skill
    compatibility.
- `internal/engine/governance/`
  - `preset_policy.go`: `ResolvePresetPolicy` (:26) and `PresetPolicy.EffectivePreset`
    (:16) — the field the plan/review/ship facets read for fail-closed enforcement;
    light fixes `MaxPlanAuditIterations=2`.
- `internal/engine/gate/`
  - `gate.go`: `EvaluateGPlan` (:66) blocks when any reason codes (incl. the
    self-audit blocker) are present; emits `plan_audit_failed` separately.
- `internal/engine/skill/`
  - `skill.go`: `SelectedReviewSkills` (:34) defines the mandatory trio:
    - `spec-compliance-review`
    - `code-quality-review`
    - `independent-review`
    and appends `security-review` when selected. `defaultGovernanceRegistry`
    (:93-124) declares all four as `StateS3Review`; `RequiredSkillsForState...`
    (:152/169) filters requiredness from the same selection.
- `cmd/`
  - `evidence.go`: the producer. `makeEvidenceSkillCmd` ("evidence skill") stamps
    engine-owned `Timestamp`/`RunVersion`, passes host `--reference` values
    verbatim into the saved record, and rejects unselected security-review evidence
    as not currently selected. When a selected reviewer already has passing
    evidence but its `context_origin:stage=review=<handle>` is malformed or
    retired, `evidence skill` allows the same reviewer to restamp fresh evidence
    instead of dead-ending behind `evidence_skill_not_current`.
  - `next_skill_view.go`, `status_view_build.go`, `validate.go`, `review.go`,
    stale-evidence recovery, and fixtures expose `selected_review_skills` while
    preserving a conventional `Name` for single-skill consumers.
- `internal/tmpl/templates/skills/` (t-05, DONE)
  - `spec-compliance-review/SKILL.md.tmpl`: host-native subagent dispatch section;
    emits `context_origin:stage=review=<handle>`.
  - `code-quality-review/SKILL.md.tmpl`: emits
    `context_origin:stage=review=<handle>`.
  - `independent-review/SKILL.md.tmpl`: promoted standalone S3 host; emits
    `context_origin:stage=review=<handle>`.
  - `security-review/SKILL.md.tmpl`: selected-security S3 host; emits
    `context_origin:stage=review=<handle>` when dispatched.
  - `goal-verification/SKILL.md.tmpl` (:88-95,188): emits
    `context_origin:stage=review=<handle>` because goal-verification is folded
    into the selected S3 reviewer set.
  - `final-closeout/SKILL.md.tmpl` (:122-149,161-181): no longer emits a
    `context_origin:stage=closeout=<handle>` token; RETAINS the engine-consumed
    `closeout:reviewer_independence=pass` (#239, REQUIRED on standard/strict) and
    documents the always-on chain-order invariant.
  - `plan-audit/SKILL.md` (static, not .tmpl, :169-203): emits the author/auditor
    PAIR `plan_origin:<handle>` + `audit_origin:<handle>` (NOT a
    `context_origin:stage=` token).
  - `wave-orchestration/SKILL.md.tmpl`: the existing force-parallel host-native
    executor fan-out (per-task `executor_agent:wave=...:task=...:<handle>`) the
    review/verify/closeout templates explicitly say to model on.
- `internal/tmpl/`
  - `templates_test.go`: token-contract tests
    (`TestSpecComplianceReviewTemplateEmitsContextOriginHandle`,
    `TestCodeQualityReviewTemplateEmitsContextOriginHandle`,
    `TestPlanAuditTemplateEmitsPlanAndAuditOriginHandles`,
    `TestGoalVerificationTemplateEmitsContextOriginHandle`,
    `TestFinalCloseoutTemplateEmitsContextOriginHandle`,
    `TestFinalCloseoutTemplateRequiresReviewerIndependenceAndChainOrder`) plus
    selected-review token tests and NotContains guards proving retired review-origin
    / review-context token forms are gone.
- `internal/toolgen/`
  - `toolgen_test.go`: generated-skill contracts;
    `TestResolveNextSkillOutputsMapToExportedHostSkills` maps the selected review
    outputs to exported host skills; wave fan-out pins
    `TestWaveOrchestrationSkillForcesParallelByDefault`.
- `internal/architecture/`
  - `dependency_direction_test.go`: forbids internal/model + internal/state
    importing cmd/tmpl/toolgen (new gates -> progression, vocab -> model).
- `docs/` (t-07, DONE)
  - `design.md`, `workflow.md`: document the selected review set, shared
    `stage=review` token, skill-keyed R2 lattice, selected-set chain order,
    standard/strict-error vs light-advisory behavior, fail-closed recovery, and
    structural-not-cryptographic trust tier.
- `artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/`
  - `intent.md`, `research.md`, `requirements.md`, `decision.md`, `tasks.md`,
    `assurance.md`, `HANDOFF.md`: the governed bundle for #240. NOTE: `tasks.md`
    checkbox state can lag implementation; use current code plus this refreshed
    codebase map for selected-review-set behavior. `research.md` still documents
    the old `review_origin` baseline in several places as pre-change input (what
    was removed, not current state), while `decision.md` and the current source own
    the variable review-set design.
