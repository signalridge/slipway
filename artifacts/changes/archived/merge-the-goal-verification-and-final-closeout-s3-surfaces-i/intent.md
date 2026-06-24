# Intent

## Summary
Merge the goal-verification and final-closeout S3 surfaces into a single pre-ship verification stage. Direction B from design analysis: the two are the only S3 skills with run_tests; both verify completion; final-closeout already reuses goal-verification's suite proof, so they are one verification concern artificially split into two peers. The merged surface owns suite-result.yaml production, acceptance-criteria (3-level) proof, guardrail high_risk_check SAST baseline, and the final pre-ship freshness/assurance/independence closeout stamp, gated hard on G_ship. Retire the proof-reuse-edge machinery between them (closeoutGoalVerificationReuseBlockers / proofReuseEdge), the goal<->closeout chain-order invariant, and the now-dead reason codes (closeout_goal_verification_reuse_invalid, closeout_chain_order_invalid). Reclassify the surface out of the review taxonomy (IsReviewSkill / reviewSkillGoal): it is verification that runs tests, not an adversarial read like spec/code/independent/security. Keep all fail-closed properties; this is a breaking change to the governance evidence contract, CLI JSON, generated skills, and docs.

## Complexity Assessment
complex
<!-- Rationale: breaking change to the public governance evidence contract and the fail-closed G_ship gate; touches the skill registry, ship-authority evaluation, reason-code catalog, every adapter's generated skills, and docs. Driven at strict preset. -->

## Guardrail Domains
<!-- none detected -->
No runtime guardrail domain (not auth/credentials/financial/schema/external-API in the runtime sense). It is nonetheless a sensitive governance-contract change to the fail-closed ship gate, handled at the strict workflow preset.

## In Scope
The single surviving terminal verification gate is named **`ship-verification`**. It runs LAST in S3, after the adversarial review peers, is always-required (not conditional), is NOT a review skill, and is the sole hard G_ship gate. It owns: one authoritative fresh full-suite run, guardrail `high_risk_check:<domain>.safety_baseline` SAST proof, acceptance-criteria 3-level (Exists/Substantive/Wired) proof, evidence-freshness recheck, `assurance.md` completeness attestation, and reviewer-independence attestation.

Concrete surfaces to change:
- `internal/engine/skill/skill.go`: remove `goal-verification` + `final-closeout` registry entries and the `reviewSkillGoal` constant; add `ship-verification` (State S3, always-required, HardGate `G_ship`, AllowedOperations incl. `run_tests`); drop goal from `SelectedReviewSkills`/`IsReviewSkill` so the selected review set is spec-compliance + code-quality + independent (+ security by policy) only.
- `internal/engine/progression/constants.go`: replace `SkillGoalVerification`/`SkillFinalCloseout` with `SkillShipVerification`.
- `internal/engine/progression/authority.go`: delete `proofReuseEdge`/`proofReuseEdgeBlockers`/`closeoutGoalVerificationReuseBlockers` and the goal<->closeout `closeoutChainOrderBlockers`; rebuild ship-authority so `ship-verification` is the single terminal gate; move the assurance + reviewer-independence attestations onto it; keep a single ordering invariant "ship-verification timestamped at/after every selected review peer".
- `internal/model/reason_code.go`: retire `closeout_goal_verification_reuse_invalid` and `closeout_chain_order_invalid`; re-home `closeout_assurance_attestation_missing` / `closeout_reviewer_independence_missing` / `verification_evidence_missing` onto the ship-verification gate (rename to `ship_verification_*` where it reads clearer).
- `internal/tmpl/templates/skills/`: delete `goal-verification/` and `final-closeout/` templates; add `ship-verification/`; drop the shared `suite-result.yaml` keystone from `spec-compliance-review`, `code-quality-review`, `independent-review`, `security-review` templates (reviewers read code/artifacts + S2 execution evidence; they no longer consume a goal-produced suite-result); re-point `coverage-analysis` from the goal-verification host to the ship-verification host.
- `cmd/next_skill.go`: re-wire `RequiredHighRiskTokens` from `SkillGoalVerification` to `SkillShipVerification`.
- `cmd/done.go` + ship-gate path: G_ship continues to require fresh `ship-verification` evidence.
- `internal/engine/progression/evidence_digests.go`: replace goal-verification + final-closeout digests with a single ship-verification digest; remove suite-result keystone digest inputs.
- Generated adapter skills/command surfaces (regenerated via `internal/toolgen`): all adapters reflect `ship-verification`; no dangling `goal-verification`/`final-closeout` skill dirs.
- `docs/` (workflow.md, design.md): update the S3 selected review set and the "what counts as complete" completion model.

## Out of Scope
- S2 `wave-orchestration` execution mechanics: the shared suite keystone is cancelled, NOT moved to S2, so S2's run_tests/execution-summary behavior is untouched.
- The substance of the adversarial review skills (spec/code/independent/security mandates) beyond removing the cancelled suite-result keystone dependency.
- Any backward-compat / migration shim: clean break. In-flight changes already at S3 with old split evidence must re-run S3 under the new flow.
- Renaming or refactoring ReasonCodes unrelated to the goal/closeout merge.

## Constraints
- Fail-closed preserved: `ship-verification` is a hard G_ship gate with no bypass, force-close, or private attestation path.
- Clean break, no compatibility scaffolding for the retired split surfaces.
- Code, generated skills, command surfaces, docs, and agent instructions stay aligned as one product surface.
- `go test ./...` green across all packages.

## Acceptance Signals
- A governed change's S3 `selected_review_skills` = spec-compliance-review, code-quality-review, independent-review (+ security-review by policy), with NO goal-verification.
- `ship-verification` resolves as the single always-required terminal S3 gate carrying hard `G_ship`; `goal-verification` and `final-closeout` no longer resolve as skill names anywhere (registry, templates, generated adapters, docs).
- The retired ReasonCodes `closeout_goal_verification_reuse_invalid` and `closeout_chain_order_invalid`, and the `proofReuseEdge` reuse-edge machinery, are gone from the codebase.
- The shared `suite-result.yaml` keystone is no longer produced or consumed; the authoritative suite runs once, inside ship-verification.
- `go test ./...` is green across all packages.
- DOGFOOD: this change itself reaches `done-ready` through the new `ship-verification` gate.

## Open Questions
None.

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
Confirmed by user 2026-06-23T18:21:56Z.

Merge `goal-verification` and `final-closeout` into a single terminal S3 verification gate named **`ship-verification`**.

- **Position / nature**: runs LAST in S3, after the spec / code / independent (+ security) adversarial review peers; always-required; NOT a review skill; the sole hard `G_ship` gate.
- **Responsibilities**: one authoritative full-suite run, guardrail `high_risk_check:<domain>.safety_baseline` SAST proof, acceptance-criteria 3-level (Exists/Substantive/Wired) proof, evidence-freshness recheck, `assurance.md` completeness attestation, reviewer-independence attestation.
- **Cancel shared keystone**: `suite-result.yaml` is no longer produced or consumed; review peers read code/artifacts + S2 execution evidence; the authoritative suite runs once, inside ship-verification. S2 is NOT touched.
- **Retire**: the `goal-verification` and `final-closeout` surfaces; the `proofReuseEdge` / reuse-edge machinery; the dead reason codes `closeout_goal_verification_reuse_invalid` and `closeout_chain_order_invalid`. Assurance / independence / verification-missing attestations re-home onto ship-verification.
- **Clean break**: no compatibility shim; in-flight S3 changes re-run S3 under the new flow.
- **Fail-closed preserved**: no bypass, force-close, or private attestation path.
- **In scope / out of scope**: as enumerated above. **Primary acceptance signal**: a change's `selected_review_skills` no longer contains goal-verification, `ship-verification` is the single terminal `G_ship` gate, the dead machinery/codes are gone, `go test ./...` is green, and this change itself reaches `done-ready` through the new gate (dogfood).
- **Out-of-scope marker**: S2 wave-orchestration execution mechanics and any backward-compat migration shim are explicitly excluded.
