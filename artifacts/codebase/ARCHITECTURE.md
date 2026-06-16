# Architecture

Re-authored for change
`add-an-engine-consumed-context-origin-fresh-context-attestat`.

## Affected Seams

- `internal/engine/progression/authority.go` — the verdict-gate seam.
  - `evaluateReviewAuthorityWithPolicy` builds `ReviewAuthority`
    (`PassingSkills` keyed by skill name, including `SkillSpecComplianceReview`
    and `SkillCodeQualityReview`); this is where review-pair handle/dispatch
    tokens would be consumed (P2's read side).
  - `buildShipAuthorityFromReadiness` is the ship-stage gate. It already holds
    `reviewAuthority.PassingSkills` AND `verifyPassingSkills` (goal +
    final-closeout) in one scope, and computes the standard/strict predicate
    `assuranceRequired := inputs.Policy.EffectivePreset != model.WorkflowPresetLight`.
    This is the natural home for P1's always-on chain-ordering gate and its
    preset-keyed error/advisory split.
  - Pattern A: `closeoutAssuranceAttestationBlockers` (presence attestation —
    `assuranceCompleteReference = "closeout:assurance_complete=pass"` must be
    in the final-closeout `References`).
  - Pattern B: `closeoutGoalVerificationReuseBlockers`, whose
    `closeoutGoalVerificationReuseReviewBlocker` already enforces
    `goal.Timestamp >= max(spec-compliance-review, code-quality-review)` BUT
    only inside the opt-in `closeout:goal_verification_reuse=pass` branch. P1
    promotes that ordering to always-on (closeout >= goal >= max(reviews)) with
    a distinct reason code, giving the dead
    `closeout:reviewer_independence=pass` token a consumer.
- `internal/engine/progression/wave_sync.go` — the dispatch-machinery seam P2
  reuses for the review pair. `DispatchEvidenceBlockers` (parallel wave missing a
  valid `dispatch_mode`) and `ExecutorAgentBlockers` (parallel_subagents wave
  missing per-task `executor_agent` handle) are the analogues to clone for the
  review pair. `SyncGovernedWaveExecution` / `evaluateGovernedWaveExecution`
  aggregate these as `safetyNetBlockers` and are currently **preset-agnostic** —
  P4's #5/#6 must thread `EffectivePreset` through here. `session_isolation_warning`
  is emitted in `LoadExecutionTasksFromEvidence` (advisory, task-only, empty
  `session_id` excluded) — the token P4 demotes.
- `internal/model/verification.go` — `VerificationRecord`. `References []string`
  is the sole host-controlled inbound channel; `Verdict`/`Timestamp`/`RunVersion`
  are engine-owned. New attestation tokens ride `References`, not a new field.
- `internal/model/wave_execution.go` — `WaveDispatchModesFromVerification`,
  `ExecutorAgentHandlesFromVerification`, `WaveDispatchParallel`, and the token
  prefixes (`dispatch_mode:wave=`, `executor_agent:wave=...:task=...`). P2's
  per-review context-id handles mirror this token grammar.
- `internal/model/reason_code.go` (`canonicalReasonDefinitions`, `NewReasonCode`,
  severity map) + `internal/model/reason_code_contract_test.go`
  (`TestCanonicalReasonCodeTaxonomySnapshot`) + `internal/model/recovery.go`
  (remediation vocab) — the three-file reason-code contract any new gate code
  must register in. `NewReasonCode` silently downgrades unregistered codes to
  `unknown_reason_code`.
- `internal/engine/skill/skill.go` — the four independence skills
  (`spec-compliance-review`, `code-quality-review`, `goal-verification`,
  `final-closeout`) are `RunSummaryBound: true`, so they share one `RunVersion`;
  this is *why* RunVersion cannot discriminate context origin (P3 residual).
- `cmd/evidence.go` — the producer. `makeEvidenceSkillCmd` stamps engine-owned
  `Timestamp`/`RunVersion` (`evidenceSkillRunContext`) and passes through host
  `--reference` values verbatim into the saved record.
- `internal/tmpl/templates/skills/{spec-compliance-review,code-quality-review,
  goal-verification,final-closeout,wave-orchestration}/SKILL.md.tmpl` +
  `internal/toolgen` (`Arguments` contract in `surface_manifest.go`/`toolgen.go`)
  document/emit the tokens the gates consume.

## Dependency Flow

Host emits `--reference` tokens via `slipway evidence skill` ->
`cmd/evidence.go` stamps engine-owned `Timestamp`/`RunVersion` and writes
`verification/<skill>.yaml` -> `authority.go` consumes them at the ship/review
gate (`buildShipAuthorityFromReadiness`, `closeoutGoalVerificationReuse*`).
For waves: `slipway evidence task` -> per-task JSON ->
`SyncGovernedWaveExecution` -> `dispatch_mode`/`executor_agent` tokens parsed by
`wave_execution.go` -> `DispatchEvidenceBlockers`/`ExecutorAgentBlockers`. New
chain-ordering and review-pair gates go in `internal/engine/progression`; new
reason-code vocabulary stays pure in `internal/model`.

## Constraints And Invariants

- Engine is the sole verdict/freshness stamper; the host may add `References`
  but cannot stamp `Timestamp`/`RunVersion`. Honest hybrid: host-emitted handles
  are audit/structural-tier evidence, not crypto proof.
- References-only, additive: no new `VerificationRecord` struct field, avoiding
  Lattice/JSON-schema and toolgen `Arguments` contract churn.
- Fail-closed on standard/strict (`EffectivePreset != WorkflowPresetLight`),
  advisory on light — the same predicate `closeoutAssuranceAttestationBlockers`
  uses. `SyncGovernedWaveExecution` must learn this preset (today it is
  preset-blind).
- Three-file reason-code contract: every new code must land in
  `canonicalReasonDefinitions` + the snapshot/severity test + `recovery.go`
  remediations, or `NewReasonCode` silently downgrades it.
- Layering ban: `internal/architecture/dependency_direction_test.go`
  (`TestAuthorityPackagesDoNotImportSurfaceRenderers`) forbids `internal/model`
  and `internal/state` from importing `cmd`, `internal/tmpl`, or
  `internal/toolgen`. New gates go in `internal/engine/progression`; new vocab
  stays in `internal/model`.
- RunSummaryBound parity gives all four independence skills the same RunVersion,
  so RunVersion cannot prove fresh-context origin — P3's documented residual;
  cross-stage timestamp ordering (Pattern B) is the strongest honest discriminator.
