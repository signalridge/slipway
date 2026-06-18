# Architecture

Re-authored for change `resolve-current-open-issues` (#263/#170/#169/#168/#167/#161).

## Current Change Focus: Open Issue Batch

This change spans five implementation seams and one tracker/documentation seam:

- Evidence recovery: `cmd/evidence.go` validates whether a skill can be recorded.
  In S3 it rejects an already passing selected reviewer unless
  `selectedReviewContextOriginRefreshRequired` or an active review alignment fix
  allows replacement (`cmd/evidence.go:1078-1110`). The ship/review authority can
  still fail closed when a present passing selected reviewer lacks a valid
  `context_origin:stage=review=<handle>` (`internal/engine/progression/authority.go:828-845`,
  `:1002-1006`). The recovery path belongs in the public evidence/fix command
  surfaces, not in hand-edited verification files.
- Adapter generation: `internal/fsutil/transaction.go` already exposes
  `ApplyFileTransaction` with rollback (`:47-52`, `:149-172`), but
  `internal/toolgen/toolgen.go` still emits skills, commands, support files,
  indexes, hooks, and the sentinel through repeated `writeDeterministic` calls
  and performs cleanup before the write pass (`:824-1072`, `:1102-1125`). This is
  a generation transaction/ownership boundary, not a new filesystem framework.
- Generated-surface ownership: `internal/toolgen/surface_manifest.go` defines a
  public inventory row with kind/name/source/docs/token only (`:21-29`). A new
  adapter ownership manifest can be separate from the public surface manifest so
  it can track path and sha256 without changing the public docs inventory shape.
- Install profiles/routers: generated skills are currently selected from
  hardcoded governance/template/standalone/technique/catalog slices in
  `internal/toolgen/toolgen.go` (`:840-1005`). A profile closure should be
  computed over this existing registry and must never prune lifecycle-critical,
  gate-owning, or sensitive-domain review skills.
- Docs: `docs/` and `mkdocs.yml` are flat. Diataxis can be added by moving or
  adding docs under tutorials/how-to/reference/explanation while keeping
  command/surface manifest contracts intact. GSD Core's local docs tree provides
  the Diataxis structure and two-tutorial baseline; Trellis provides a practical
  Start Here model with first-task setup, how-it-works runtime flow, real-world
  scenarios, and task/spec/memory concepts that fit Slipway onboarding.
- Test quality: `.golangci.yaml` enables only stock linters (`:7-12`). A
  Go-native analyzer package can live under `internal/testlint` with a small CLI
  for CI and targeted tests, while policy text belongs in `docs/contributing.md`.

### Boundaries

- Preserve `internal/model` as pure contract vocabulary and keep lifecycle
  policy in `internal/engine/progression`.
- Keep toolgen file ownership separate from the public `docs/SURFACE-MANIFEST.json`.
- Treat generated adapters as user-adjacent output: unknown or modified files are
  preserved or backed up, not silently deleted.
- Do not copy GSD-core internals. Borrow the mechanism shape only where it fits
  Slipway's Go toolgen and fail-closed lifecycle.

Re-authored for change `generalize-digest-proof-reuse` (#258).

## Current Change Focus: Digest-Keyed Proof Reuse

The active change touches the S3/S4 verification authority path, not the
subagent-dispatch architecture documented below for #240. The primary seam is
`internal/engine/progression/authority.go`: `buildShipAuthorityFromReadiness`
loads fresh goal-verification and final-closeout records, then appends
`closeoutGoalVerificationReuseBlockers` into both `VerifySkillBlockers` and the
unresolved ship-gate reason set so a reuse failure reaches `G_ship` as
`closeout_goal_verification_reuse_invalid`.

The current reuse implementation is closeout-specific. It is activated only when
final-closeout records `closeout:goal_verification_reuse=pass`, then validates
the reuse run version against goal-verification, final-closeout, and
execution-summary, verifies execution-summary freshness, and rechecks both
goal-verification and final-closeout digest inputs. The helpers that collect
changed and target file content now use proof-reuse-neutral names:
`proofReuseContentPaths`, `proofReuseWorkspacePaths`, and
`proofReuseSkipsContentPath`. The reuse run-version reference parser remains
closeout-token-specific as `closeoutGoalVerificationReuseRunVersion`.

The proof substrate already exists outside the closeout-specific gate:
`internal/model/evidence_digests.go` defines `SuiteResult` with
`run_summary_version`, `full_suite_digest`, and optional `sast_digests`, while
`SharedReviewerInputDigests` exposes those values as digest inputs. In
`internal/engine/progression/evidence_digests.go`, goal-verification already
uses suite-result, planning artifacts, task-plan scope, changed/target file set,
and changed/target file content as freshness inputs. This makes the smallest
architecture change an extraction/generalization of the existing validation
shape, not a second proof system.

### Boundaries

- Keep vocabulary and low-level proof structs in `internal/model` only when they
  are external/data contracts.
- Keep lifecycle and ship-gate policy in `internal/engine/progression`.
- Keep host behavior in `internal/tmpl/templates/skills/*`.
- Do not copy GSD-core's broader orchestration architecture; it is useful
  background for context isolation and bounded handoff, but #258 is about
  reusing already-digest-fresh proof inside Slipway's existing lifecycle gates.

Re-authored for change
`feat-governance-host-native-subagent-enforced-cross-stage-in` (#240).

Baseline: #239 (engine-consumed reviewer-independence + context-origin/`review_origin`
attestation, closeout chain-ordering, and the wave dispatch/executor gates) has
SHIPPED (commit 2d2adac, 0.26.0); this change builds on that baseline and retires
the per-review-pair `review_origin` grammar in favor of the chain-wide
`context_origin:stage=` independence lattice. This refresh supersedes the stale
S3 pair plan: the current worktree routes one selected review set with a
mandatory spec/code/independent trio and an optional security reviewer selected
from governance controls.

`review_origin` is RETIRED with no compat shim: no `ReviewOriginReferencePrefix`,
`ReviewOriginHandle`, `ReviewOriginHandleFromVerification`, or
`review_origin_handle_invalid` symbol exists in source. Independence is now a
variable participant lattice over the author -> selected-review -> verify -> close
chain.

## Affected Seams

- `internal/model/context_attestation.go` — the pure attestation grammar and
  lattice (t-01, DONE). stdlib-only (`sort`/`strconv`/`strings`), no
  cmd/tmpl/toolgen import.
  - Token grammar: `ContextOriginReferencePrefix = "context_origin:stage="`
    (:38), shape `context_origin:stage=<stage>=<handle>`;
    `parseContextOriginReference` (:86) splits on the FIRST `=` after the prefix,
    so a `=` inside the handle survives. `PlanOriginReferencePrefix =
    "plan_origin:"` (:43) and `AuditOriginReferencePrefix = "audit_origin:"`
    (:48) take the whole remainder as the handle (`parseStageOriginReference`,
    :145).
  - Six canonical stage constants (:22-28): `StageContextExecutor` `"executor"`,
    `StageContextPlanOrigin` `"plan_origin"`, `StageContextAuditOrigin`
    `"audit_origin"`, `StageContextReview` `"review"`, `StageContextGoal`
    `"goal"`, and `StageContextCloseout` `"closeout"`. Review records all use
    the shared `review` wire stage; the authority feeder supplies per-skill
    participant keys.
  - `ContextOriginHandle{Stage, Handle}` (:53) is the parsed token;
    `ContextParticipant{Handle, HandleSet}` (:181) is a stage's lattice
    contribution — exactly one populated (single-handle stages set `Handle`, the
    executor stage sets `HandleSet`).
  - Extractors fail closed on ambiguity, idempotent on repeat:
    `ContextOriginHandlesFromVerification` (:65) keys per stage and returns
    `(nil,false)` if two refs name one stage with different handles;
    `PlanOriginHandleFromVerification` (:111) / `AuditOriginHandleFromVerification`
    (:120) delegate to `singleStageHandleFromVerification` (:124);
    `ExecutorParticipantHandleSetFromVerification` (:164) flattens the per-wave
    per-task map from `ExecutorAgentHandlesFromVerification`
    (`wave_execution.go:336`) into a deduped set, dropping the blank conflict-collapse
    value, and is never nil.
  - `CrossStageContextCollisions(participants, ownedStages)` (:194) is the pure
    pairwise collision detector: returns each colliding stage pair once as a
    lexically ordered `[stageA, stageB]` tuple, sorted deterministically, keeping
    an edge only when at least one endpoint is in `ownedStages`.
    `participantsCollide` (:218) is the predicate: two single-handle stages
    collide iff handles are equal; a single-handle stage collides with the
    executor stage iff its handle is a member of the executor `HandleSet`.
  - Pre-existing sibling grammar `DegradedDispatchJustificationsFromVerification`
    (:248) + `WaveDegradedJustificationReferencePrefix` (:240) live in this file
    but are an unrelated wave-dispatch concern, unaffected by #240.
- `internal/engine/progression/authority.go` — the review+ship verdict-gate seam
  that CONSUMES the lattice (t-03, DONE).
  - `evaluateReviewAuthorityWithPolicy` (:79) is the S3 review gate. It derives
    the security-review selector, computes the selected review-skill slice, runs
    `EvaluateRequiredSkillsForChangeWithReviewSelection`, and appends
    cross-stage blockers over the selected review participants; required-flag is
    `EffectivePreset != light`.
  - `buildShipAuthorityFromReadiness` (:140) is the S4 ship gate. It re-loads the
    review participants via `mergeContextHandleRecords` (:624), adds goal +
    closeout, and DUAL-SURFACES the lattice/closeout blockers into both
    `VerifySkillBlockers` and the unresolved set feeding `EvaluateGShip`, so the
    specific code reaches G_ship reasons rather than collapsing to
    `verification_evidence_missing`.
  - `crossStageContextParticipants` (:525) builds participants: executor handle
    set from `LatestPassingWaveEvidence`, `audit_origin` from the plan-audit
    record, selected review records from `passingSkills` keyed by skill name and
    parsed from `context_origin:stage=review=<handle>`, plus goal/closeout from
    their own stage tokens. Absent/non-passing is silent; present-passing with no
    well-formed handle fails closed with `context_origin_handle_invalid`.
  - `crossStageContextDistinctBlockers` (:657) builds participants, short-circuits
    to `context_origin_handle_invalid` on a malformed handle, else emits one
    `cross_stage_context_not_distinct` per colliding owned-endpoint pair (detail
    `earlier|later`, lexical order). Returns nil on light.
  - Owned-edge partition: review owns `{executor, audit_origin}` plus every
    selected review skill (`crossStageContextOwnedReviewStagesForSelectedSkills`,
    :487), while ship owns `{goal, closeout}` (:505) against the same selected
    review base. Mandatory selection owns 10 review edges and 11 ship edges;
    selected security expands that to 15 review edges and 13 ship edges. No edge
    double-fires. Load-sets
    `crossStageContextReviewStagesForSelectedSkills` (:691) /
    `crossStageContextShipStagesForSelectedSkills` (:699).
  - Closeout facets, all preset-gated (`EffectivePreset != light`):
    `closeoutAssuranceAttestationBlockers` (:262, #47),
    `closeoutReviewerIndependenceBlockers` (:379, REQ-001 presence facet),
    `closeoutChainOrderBlockers` (:430, always-on relative to the opt-in
    `goal_verification_reuse` token but still preset-gated; every selected review
    verdict must be at or before goal verification, and final closeout must not
    predate goal; distinct code `closeout_chain_order_invalid`). The
    review<=goal / closeout>=goal halves moved OUT of the opt-in
    `closeoutGoalVerificationReuseBlockers` (:285) into the always-on chain-order
    gate.
- `internal/engine/progression/advance_governed.go` — the S1 PLAN gate and the
  local plan-audit self-audit edge (t-04, DONE).
  - `EvaluatePlanGate` (:958) is preset-aware:
    `func EvaluatePlanGate(root string, change model.Change, passingSkills map[string]model.VerificationRecord, presetPolicy governance.PresetPolicy) gate.GateEvaluation`.
    It is called on both the advance path (`CheckGateWithIteration`, :582 ->
    `EvaluatePlanGate` at :588, preset resolved at :587) and the read/readiness path
    (`readiness.go` `evaluateGateReadiness` -> :488, preset resolved at :487), so
    status and readiness cannot diverge.
  - `planAuditOriginHandleBlockers` (:996) owns ONLY the local edge: on a present,
    passing plan-audit record it requires a well-formed `plan_origin` AND
    `audit_origin` with `plan_origin != audit_origin`; missing either or equal
    -> `plan_audit_origin_invalid` via `planAuditOriginInvalidBlocker` (:1021).
    `enforced := presetPolicy.EffectivePreset != model.WorkflowPresetLight`
    (:970); not enforced (light) returns nil (advisory). An ABSENT plan-audit
    record yields `plan_audit_evidence_missing` instead. The plan gate adds NO
    cross-stage rung whose other endpoint is absent at S1.
- `internal/model/reason_code.go` (`canonicalReasonDefinitions`, :40) +
  `internal/model/reason_code_contract_test.go`
  (`canonicalReasonCodeSnapshot` :41, `canonicalReasonSeveritySnapshot` :218,
  `TestCanonicalReasonCodeTaxonomySnapshot`) + `internal/model/recovery.go`
  (`blockerRemediations`, :103) + `internal/model/recovery_test.go`
  (`inScopeProducedRecoverySpecs` :131, `TestInScopeProducedBlockersResolveToCanonicalRecovery`)
  — the reason-code contract (t-02, DONE). It is now effectively a FOUR-file
  contract: `recovery_test.go` directly enforces the three new codes resolve to a
  canonical message + non-empty remediation + non-empty command.
  - NEW codes, all `ReasonSeverityError`: `context_origin_handle_invalid`
    (`reason_code.go:501`, renamed from the retired `review_origin_handle_invalid`,
    widened from the old review scope to every stage),
    `cross_stage_context_not_distinct` (:505), `plan_audit_origin_invalid` (:509).
    Each has a matching `RecoveryClassRerunSkill` + `slipway run` remediation
    (`recovery.go:517/522/530`) routing the operator to re-run the owning stage in
    a fresh native subagent.
  - `TestReviewOriginHandleVocabularyRetired` (`reason_code_contract_test.go:274`)
    asserts `review_origin_handle_invalid` is absent from the registry, absent from
    remediations, and unrecognized by `IsCanonicalReasonCode`.
- `internal/model/verification.go` — `VerificationRecord`. `References []string`
  is the sole host-controlled inbound channel; `Verdict`/`Timestamp`/`RunVersion`
  are engine-owned. Every new token rides `References`, not a new field or CLI
  flag.
- `internal/model/wave_execution.go` — `ExecutorAgentHandlesFromVerification`
  (:336) and the `executor_agent:wave=...:task=...` grammar are reused unchanged;
  the executor stage is the only set-valued lattice participant.
- `internal/engine/skill/skill.go` — selected review-set definition and required
  skill filtering. `SelectedReviewSkills` (:34) returns the mandatory trio:
  - `spec-compliance-review`
  - `code-quality-review`
  - `independent-review`
  and appends `security-review` when `ReviewSkillSelection.SecurityReviewSelected`
  is true. The default registry now includes all four as `StateS3Review`
  definitions (:93-124); `RequiredSkillsForStateWithRegistryWithReviewSelection`
  (:169) filters requiredness from the same selection.
- `internal/engine/progression/skill_resolution.go` — `ResolveNextSkill` (:23),
  the next-action dispatcher. It now returns a skill slice; `S3_REVIEW` returns
  `skill.SelectedReviewSkills(reviewSelection)` (:39-42), so the selected review
  peers dispatch concurrently and none precedes another. `PrimaryNextSkill` (:51)
  is only a compatibility projection for callers that truly need one conventional
  skill and must not be treated as S3 ordering authority.
- `internal/tmpl/templates/skills/` — the review, goal, closeout, and plan-audit
  host-facing dispatch surfaces. The selected review hosts instruct native
  subagent dispatch on the SHARED change worktree and record
  `context_origin:stage=review=<handle>`; goal/closeout retain their own
  `stage=goal` / `stage=closeout` tokens, and plan-audit keeps the
  `plan_origin` + `audit_origin` pair. Token contracts are pinned in
  `internal/tmpl/templates_test.go`, including NotContains guards on retired
  review-origin / review-context token forms.

## Dependency Flow

Each governed stage's host-native subagent runs on the shared worktree and
returns claims only; the host inspects the written files and records a verdict via
`slipway evidence skill`, stamping a self-describing handle token onto
`References`: `context_origin:stage=review=<handle>` for every selected reviewer
including goal-verification,
`plan_origin:<handle>` + `audit_origin:<handle>` for the plan-audit author/auditor
pair, and the reused `executor_agent:` grammar for the S2 wave executor set.
`cmd/evidence.go` stamps engine-owned `Timestamp`/`RunVersion`, writes
`verification/<skill>.yaml`, and allows a selected reviewer to restamp when its
passing evidence has an invalid or retired review context-origin token.
`internal/model/context_attestation.go` parses those tokens fail-closed and
exposes `CrossStageContextCollisions`. The plan gate
(`advance_governed.go` `EvaluatePlanGate`) consumes the local
`plan_origin`/`audit_origin` pair; the review and ship gates (`authority.go`
`evaluateReviewAuthorityWithPolicy` / `buildShipAuthorityFromReadiness`) build the
selected-set lattice and fail closed on missing or colliding handles. New gate
wiring stays in `internal/engine/progression`; new vocabulary stays pure in
`internal/model`.

## Constraints And Invariants

- Engine is the sole verdict/freshness stamper; the host may add `References` but
  cannot stamp `Timestamp`/`RunVersion` and the engine forks no process. Honest
  hybrid: host-emitted handle strings are structural-tier attestation compared for
  distinctness (`participantsCollide`), never cryptographic proof.
- References-only, additive: no new `VerificationRecord` struct field and no new
  CLI flag — every token rides the existing `--reference`, avoiding Lattice/JSON
  and toolgen `Arguments` churn.
- Fail-closed on ambiguity: conflicting handles for the same key return
  `ok=false` rather than letting the last reference win; idempotent on identical
  repeats. A present-passing record with no well-formed handle ->
  `context_origin_handle_invalid` (and short-circuits collision evaluation); an
  absent or non-passing record contributes nothing (its absence is owned by the
  required-skill-missing gate).
- Preset floor: every #240/#47/#239 facet is error on standard/strict
  (`EffectivePreset != model.WorkflowPresetLight`) and advisory (returns nil) on
  light. Preset-resolution failure falls back to the zero `PresetPolicy`, whose
  `EffectivePreset` is `""` (!= light), so the gate ENFORCES rather than relaxing.
- Exactly-once edge partition keyed on the later-resolving endpoint: Plan gate
  owns the one local `audit_origin != plan_origin` edge. Review owns every edge
  among `{executor, audit_origin}` plus the selected review-skill keys. Ship owns
  only the new edges introduced by `{goal, closeout}` against that same selected
  review base. Mandatory selection yields 10 review-owned edges and 11 ship-owned
  edges; selected security yields 15 and 13. `CrossStageContextCollisions` keeps
  only owned-endpoint edges, so no edge double-fires.
- Executor joins as set-disjointness: each single-handle stage must be absent from
  the executor `HandleSet`; an empty set is silent; the blank conflict-collapse
  handle is dropped so it never spuriously collides. Executor-internal distinctness
  stays owned by the existing `executor_agent_missing` wave gate.
- Always-on chain order (`closeout_chain_order_invalid`): closeout >=
  goal-verification >= every selected review verdict, compared only between
  present+passing+non-zero-timestamp records and independent of the opt-in
  `goal_verification_reuse` token. Selected reviewers remain unordered peers; no
  intra-S3 ordering gate exists.
- Reason-code contract: every code must land in `canonicalReasonDefinitions` + the
  snapshot/severity test + `recovery.go` remediations (and, for in-scope producers,
  resolve via `inScopeProducedRecoverySpecs`), or `NewReasonCode` silently
  downgrades it to `unknown_reason_code`. Retired codes must be absent from all of
  the registry, remediations, and `IsCanonicalReasonCode`.
- Layering ban: `internal/architecture/dependency_direction_test.go` forbids
  `internal/model` and `internal/state` from importing `cmd`, `internal/tmpl`, or
  `internal/toolgen`. Lattice stage identifiers are local model constants, never
  the progression skill-name constants.
- Sensitive-domain fail-closed: no bypass, force-close, or private/self-stamped
  attestation path is added. Missing selected review evidence, malformed
  `stage=review` handles, selected-set collisions, and selected-set chain-order
  violations route only through public skill re-runs and engine-stamped evidence.
