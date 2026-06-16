# Tasks

## Task List

- [x] `t-01` Add the pure `internal/model` grammar and reason-code/recovery contract: a `review_origin:` review-context handle reference token (+parser, named to avoid colliding with the existing unrelated `review_context` JSON object on the next/handoff surface) and a degraded-sequential tool-unavailable justification reference token (+parser), kept free of any cmd/tmpl/toolgen import; register one distinct canonical reason code per new blocker (closeout reviewer-independence presence, chain-ordering, review-context distinctness, test≠impl distinctness, degraded-justification) in `canonicalReasonDefinitions`, add their `blockerRemediations` naming the owning skill to re-enter, and update the snapshot/severity and recovery-completeness contract tests so no code downgrades to `unknown_reason_code`.
  - depends_on: []
  - target_files: [internal/model/context_attestation.go, internal/model/context_attestation_test.go, internal/model/reason_code.go, internal/model/reason_code_contract_test.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-02` Wire P1 and P2 into `authority.go`. P1: in `buildShipAuthorityFromReadiness` require `closeout:reviewer_independence=pass` present on the final-closeout record (Pattern-A presence) AND promote the cross-stage ordering halves (`review ≤ goal` from `closeoutGoalVerificationReuseReviewBlocker`, `closeout ≥ goal`) out from behind the opt-in `closeout:goal_verification_reuse=pass` early-return into an always-on `closeout ≥ goal ≥ max(spec,code)` invariant carrying its own distinct reason code (not `closeout_goal_verification_reuse_invalid`). P2: in `evaluateReviewAuthorityWithPolicy` add a blocker requiring both reviews to record a `review_origin:` handle and the two handles to differ. Both fail closed on standard/strict (`EffectivePreset != light`), advisory on light, dual-surfaced into the ship gate; no bypass/force-close/self-stamp path. Add unit + 3-subtest preset-gating + cross-stage tests.
  - depends_on: [t-01]
  - target_files: [cmd/lifecycle_commands_test.go, internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, internal/engine/progression/freshness_guard_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-007]

- [x] `t-03` Wire P4 into `wave_sync.go`, resolving preset internally via `governance.ResolvePresetPolicy(root, change)` so #5/#6 are error on standard/strict and advisory on light without changing `SyncGovernedWaveExecution`'s signature or its three call sites. #5: enforce that for shared `target_files` a `task_kind=test` task is structurally distinct from and dispatched before its dependent `task_kind=code` task, derived only from engine-owned `task_kind`+`target_files` (never `session_id`). #6: tighten `DispatchEvidenceBlockers` so a bare `degraded_sequential` is rejected and accepted only when paired with the new tool-unavailable justification reference token; ensure it fires on the `evidence skill` sync path (cmd/evidence.go:242) as well as advance/next. Add wave-sync tests including a preset-parametrized case and the evidence-skill trigger site.
  - depends_on: [t-02]
  - target_files: [internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-004]

- [x] `t-04` Emit and document the new tokens across generated host surfaces and docs: the `review_origin:` review-context handle on the spec-compliance-review and code-quality-review templates, the engine-consumed `closeout:reviewer_independence=pass` + chain-ordering note on final-closeout and goal-verification, and the degraded-sequential justification token on wave-orchestration; add a literal-token template test per new emitted token (cloning the assurance-attestation test shape); update `docs/workflow.md` and `docs/design.md` to explain what each token attests, on which presets it is enforced, the recovery path when a gate fails closed, and the honest residual (Option B non-forgeable discrimination is infeasible within constraints — the handle gate is audit/structural tier, not cryptographic proof).
  - depends_on: [t-03]
  - target_files: [internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl, internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl, internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates_test.go, docs/workflow.md, docs/design.md]
  - task_kind: code
  - covers: [REQ-006, REQ-007]
