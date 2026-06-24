# Tasks

## Task List

- [x] `t-01` Model package + gate emit-site: retire dead reason codes, re-home attestations, drop the suite-result keystone type. In reason_code.go remove `closeout_goal_verification_reuse_invalid` and `closeout_chain_order_invalid`, and re-home `closeout_assurance_attestation_missing` / `closeout_reviewer_independence_missing` / `verification_evidence_missing` onto the ship-verification gate (rename to `ship_verification_*` where clearer); in recovery.go drop the recovery vocab for the removed codes and repoint the rehomed ones; in evidence_digests.go remove the cancelled `SuiteResult` keystone digest type/inputs. If `verification_evidence_missing` is renamed, keep the G_ship emit site in engine/gate/gate.go and its assertion in gate_test.go aligned so no literal orphans to `unknown_reason_code`. Keep the catalog contract test exhaustive and the model package compiling.
  - depends_on: []
  - target_files: [internal/model/reason_code.go, internal/model/recovery.go, internal/model/evidence_digests.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go, internal/model/model_test.go, internal/engine/gate/gate.go, internal/engine/gate/gate_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `t-02` Skill registry + review-set membership: in skill.go remove the `goal-verification` and `final-closeout` registry entries and the `reviewSkillGoal` constant, add the always-required `ship-verification` definition (State S3, HardGate `G_ship`, AllowedOperations incl. `run_tests`, NOT a review skill), and drop goal from `SelectedReviewSkills` / `SelectedReviewSkillsForWorkflowProfile` / `IsReviewSkill` so the selected review set is spec/code/independent (+security). Update the skill tests.
  - depends_on: []
  - target_files: [internal/engine/skill/skill.go, internal/engine/skill/skill_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-03` Progression package: rename the skill constant and rebuild ship authority. In constants.go replace `SkillGoalVerification`/`SkillFinalCloseout` with a single `SkillShipVerification`; in authority.go rewrite `buildShipAuthorityFromReadiness` so `ship-verification` is the single terminal G_ship gate, delete `proofReuseEdge`/`proofReuseEdgeBlockers`/`closeoutGoalVerificationReuseBlockers` and the goal↔closeout `closeoutChainOrderBlockers`, re-home the assurance + reviewer-independence attestations onto ship-verification, drop the suite-result keystone handling, and keep one ordering invariant "ship-verification timestamped at/after every selected review peer"; in evidence_digests.go replace the goal+closeout digest inputs with a single ship-verification digest and remove the suite-result keystone inputs; rename the constant references in confirmation_boundaries.go, validation.go, and skill_resolution.go. Update the progression tests.
  - depends_on: [t-01, t-02]
  - target_files: [internal/engine/progression/constants.go, internal/engine/progression/authority.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/confirmation_boundaries.go, internal/engine/progression/validation.go, internal/engine/progression/skill_resolution.go, internal/engine/progression/authority_test.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/confirmation_boundaries_test.go, internal/engine/progression/freshness_guard_test.go, internal/engine/progression/readiness_optimization_test.go, internal/engine/progression/skill_resolution_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004, REQ-005]

- [x] `t-04` Capability registry + state persistence: in capability/registry_change_shape_verification.go re-point the coverage-analysis host-embedded binding from `goal-verification` to `ship-verification`; in state/verification.go remove the cancelled suite-result keystone persistence/load path. Update the capability and state tests.
  - depends_on: [t-01, t-02]
  - target_files: [internal/engine/capability/registry_change_shape_verification.go, internal/engine/capability/contract_absorption_test.go, internal/engine/capability/resolver_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/state/evidence_digests_test.go, internal/state/local_ignore_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-005]

- [x] `t-05` CLI command surfaces: in cmd/evidence.go remove the `slipway evidence suite-result` subcommand (the cancelled keystone) and repoint the goal/closeout skill references onto ship-verification; re-wire `RequiredHighRiskTokens` in next_skill.go to `SkillShipVerification`; rename the constant references in next_skill_view.go, next.go, stats.go; update the `--full` flag prose in new.go to the ship-verification model; keep `slipway done` failing closed on the G_ship gate (done.go). Update the affected command tests and delete the suite-result evidence test.
  - depends_on: [t-03, t-04]
  - target_files: [cmd/evidence.go, cmd/next_skill.go, cmd/next_skill_view.go, cmd/next.go, cmd/stats.go, cmd/new.go, cmd/done.go, cmd/lifecycle_commands_test.go, cmd/evidence_skill_test.go, cmd/evidence_suite_result_test.go, cmd/execution_summary_test_helper_test.go, cmd/status_view_build_test.go, cmd/status_render_test.go, cmd/next_skill_constraints_test.go, cmd/next_skill_capability_hints_test.go, cmd/progression_next_test.go, cmd/governance_gate_consistency_test.go, cmd/auto_mode_test.go, cmd/review_test.go, cmd/init_test.go, cmd/stats_test.go, cmd/validate_artifact_gate_test.go, cmd/cli_e2e_test.go, cmd/done_bulk_reason_codes_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-005, REQ-006]

- [x] `t-06` Skill templates: delete the `goal-verification/` and `final-closeout/` template trees, add the `ship-verification/` template (authoritative suite run + 3-level acceptance + high-risk SAST + freshness + assurance + independence), drop the shared suite-result keystone references from the spec/code/independent/security review templates and the evidence/new/run command partials, re-point coverage-analysis from the goal-verification host to ship-verification, and scrub the residual goal/closeout references in the coding-discipline and worktree-preflight templates. Update the template tests.
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/ship-verification/SKILL.md.tmpl, internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl, internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl, internal/tmpl/templates/skills/security-review/SKILL.md.tmpl, internal/tmpl/templates/skills/coverage-analysis/SKILL.md, internal/tmpl/templates/skills/coding-discipline/SKILL.md, internal/tmpl/templates/skills/worktree-preflight/SKILL.md, internal/tmpl/templates/_partials/command-new-body.tmpl, internal/tmpl/templates/_partials/command-run-body.tmpl, internal/tmpl/templates/_partials/command-evidence-body.tmpl, internal/tmpl/templates_test.go, internal/tmpl/thin_host_content_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-005, REQ-006]

- [x] `t-07` Toolgen + docs alignment: in toolgen.go and install_profiles.go replace the goal-verification/final-closeout skill registry entries, related-skill maps, and install profiles with ship-verification; regenerate the toolgen golden inventory and the SURFACE-MANIFEST; update docs/workflow.md, docs/design.md, docs/commands.md, docs/explanation/design.md, docs/how-to/recover-and-troubleshoot.md, and README.md to describe ship-verification as the single terminal S3 gate with no goal-verification/final-closeout/suite-result keystone references. Update the toolgen tests.
  - depends_on: [t-02, t-06]
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/install_profiles.go, internal/toolgen/testdata/skill_tree_inventory.codex.golden, internal/toolgen/toolgen_test.go, internal/toolgen/install_profiles_test.go, docs/workflow.md, docs/design.md, docs/commands.md, docs/explanation/design.md, docs/how-to/recover-and-troubleshoot.md, docs/SURFACE-MANIFEST.json, README.md]
  - task_kind: code
  - covers: [REQ-006]
