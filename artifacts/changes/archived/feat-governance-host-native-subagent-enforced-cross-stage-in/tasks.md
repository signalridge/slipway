# Tasks

## Task List

- [x] `t-00` Pin the context-origin grammar contract before implementation: assert the `context_origin`, `plan_origin`, `audit_origin`, executor handle-set, collision helper behavior, retired `review_origin` vocabulary, and new shared `StageContextReview = "review"` wire token through focused tests.
  - depends_on: []
  - target_files: [internal/model/context_attestation.go, internal/model/context_attestation_test.go, internal/architecture/dependency_direction_test.go]
  - task_kind: test
  - covers: [REQ-001]
  - acceptance: `go test ./internal/model ./internal/architecture -run 'TestContext|TestReviewOrigin|TestAuthorityPackages' -count=1` passes and proves the grammar contract before the implementation task runs.

- [x] `t-01` Implement the pure context-origin grammar extension: keep the existing `context_origin`, `plan_origin`, `audit_origin`, executor handle-set, and collision helper behavior; add only `StageContextReview = "review"` as the shared review wire token; and keep `review_origin` retired with no compatibility shim.
  - depends_on: [t-00]
  - target_files: [internal/model/context_attestation.go]
  - task_kind: code
  - covers: [REQ-001]
  - acceptance: `go test ./internal/model ./internal/architecture -run 'TestContext|TestReviewOrigin|TestAuthorityPackages' -count=1` passes and `internal/model/context_attestation.go` stays stdlib-only.

- [x] `t-02` Add the engine-derived security-review selector: introduce `ControlSecurityReview`, derive it from SAST guardrail domains, high blast radius, or strict+medium blast radius, wire preset/control modes so selected security is blocking on standard/strict and advisory on light, and update governance health/action/config tests for the new control.
  - depends_on: [t-01]
  - target_files: [internal/model/control.go, internal/model/config.go, internal/model/model_test.go, internal/engine/control/config.go, internal/engine/control/config_test.go, internal/engine/control/derive.go, internal/engine/control/derive_test.go, internal/engine/control/evaluate_test.go, internal/engine/governance/preset_policy.go, internal/engine/governance/preset_policy_test.go, internal/engine/governance/actions.go, internal/engine/governance/actions_test.go, internal/engine/governance/health_test.go, internal/engine/governance/runtime_actions.go, internal/engine/governance/runtime_actions_test.go, internal/engine/governance/snapshot_test.go]
  - task_kind: test
  - covers: [REQ-004]
  - acceptance: control tests prove SAST-domain, high-blast, strict+medium-selected, standard+medium-unselected, and no-data-degrades-medium-under-strict cases.

- [x] `t-03` Build one selected-review-skill set for routing and requiredness: add standalone S3 definitions for mandatory `independent-review` and conditional `security-review`, extend required-skill filtering with the shared security-selected input, make `ResolveNextSkill` return the mandatory trio plus selected security, and update all progression/readiness callers that consume required skills or selected next skills.
  - depends_on: [t-02]
  - target_files: [internal/engine/skill/skill.go, internal/engine/skill/skill_test.go, internal/engine/progression/constants.go, internal/engine/progression/skill_resolution.go, internal/engine/progression/skill_resolution_test.go, internal/engine/progression/evidence.go, internal/engine/progression/evidence_test.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/readiness.go, internal/engine/progression/advance_governed.go, internal/engine/progression/advance_intake.go, internal/engine/progression/stale_evidence_recovery.go, internal/engine/progression/stale_evidence_recovery_test.go]
  - task_kind: test
  - covers: [REQ-003, REQ-005, REQ-009]
  - acceptance: routing and required-skill tests compare the selected S3 set across no-security and security-selected changes, with no spec-before-code sequencing dependency.

- [x] `t-04` Re-key review-authority lattice participants to selected reviewer skill names: parse each selected reviewer's `context_origin:stage=review=` handle from `passingSkills`, derive review-owned endpoints from the same selected-review-skill slice, keep `audit_origin` and executor sourcing unchanged, and add fail-closed regressions for same-handle reviewers, missing selected review handles, and unselected security evidence on disk.
  - depends_on: [t-03]
  - target_files: [internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, internal/model/context_attestation.go, internal/model/context_attestation_test.go]
  - task_kind: test
  - covers: [REQ-005, REQ-006]
  - acceptance: `go test ./internal/engine/progression -run 'Test.*CrossStageContext|Test.*ReviewAuthority' -count=1` proves selected-review collisions block and unselected on-disk security is silent.

- [x] `t-05` Extend ship authority and closeout chain-order checks to the selected review set: merge selected review participants into ship lattice evaluation, order every selected reviewer before goal/final closeout, keep absent unselected security silent, and preserve no-double-fire behavior for review-owned edges at ship.
  - depends_on: [t-04]
  - target_files: [internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, internal/engine/progression/readiness.go, internal/engine/progression/freshness_guard_test.go]
  - task_kind: code
  - covers: [REQ-007, REQ-010]
  - acceptance: authority tests cover independent/security review ordering, selected-security ship participation, unselected-security silence, and no re-fire of review-owned edges at ship.

- [x] `t-06` Promote independent-review and security-review to workflow-owned host templates and de-embed review checklists: add templated S3 host surfaces that dispatch native subagents on the shared worktree and record `context_origin:stage=review=<handle>`, remove independent/security host-embedded bindings from spec/code, preserve command-auto bindings, update toolgen descriptors/allowlists/goldens, and update template tests.
  - depends_on: [t-03]
  - target_files: [internal/engine/capability/registry_default.go, internal/engine/capability/registry_b2.go, internal/engine/capability/registry.go, internal/engine/capability/registry_test.go, internal/engine/capability/resolver_test.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest_test.go, docs/SURFACE-MANIFEST.json, internal/tmpl/templates/skills/independent-review/SKILL.md, internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl, internal/tmpl/templates/skills/independent-review/PROSE.tmpl, internal/tmpl/templates/skills/independent-review/CHECKLIST.tmpl, internal/tmpl/templates/skills/independent-review/VERDICT.tmpl, internal/tmpl/templates/skills/security-review/SKILL.md, internal/tmpl/templates/skills/security-review/SKILL.md.tmpl, internal/tmpl/templates/skills/security-review/CHECKLIST.tmpl, internal/tmpl/templates/skills/plan-audit/SKILL.md, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl, internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-008, REQ-009, REQ-010]
  - acceptance: template/toolgen tests prove both promoted hosts export as workflow-owned S3 skills, all reviewers emit `stage=review`, and spec/code no longer embed the independent base-reader procedure.

- [x] `t-07` Update public CLI review-set surfaces and fixtures: teach `next`, `status`, `validate`, `evidence`, `review`, and stale-evidence recovery to show or reason over the selected review set; remove remaining spec-before-code actionable fallbacks; update command tests and fixtures for mandatory trio and selected-security quartet behavior.
  - depends_on: [t-03, t-04, t-06]
  - target_files: [cmd/next.go, cmd/next_skill.go, cmd/next_skill_view.go, cmd/validate.go, cmd/evidence.go, cmd/review.go, cmd/stats.go, cmd/status.go, cmd/status_view_build.go, cmd/next_handoff.go, cmd/evidence_skill_test.go, cmd/evidence_task_test.go, cmd/governance_gate_consistency_test.go, cmd/next_skill_constraints_test.go, cmd/next_skill_capability_hints_test.go, cmd/progression_next_test.go, cmd/status_view_build_test.go, cmd/review_test.go, cmd/stats_test.go, cmd/lifecycle_commands_test.go, internal/tmpl/templates/_partials/command-run-body.tmpl, internal/engine/progression/stale_evidence_recovery.go, internal/engine/progression/stale_evidence_recovery_test.go]
  - task_kind: test
  - covers: [REQ-003, REQ-005, REQ-009]
  - acceptance: command tests assert S3 missing-review blockers and handoff JSON identify all selected reviewers and do not make code-quality-review depend on spec-compliance-review.

- [x] `t-08` Maintain reason-code and recovery completeness for review-SET blockers: reuse existing `required_skill_missing`, `context_origin_handle_invalid`, `cross_stage_context_not_distinct`, `plan_audit_origin_invalid`, and `closeout_chain_order_invalid` where possible; if a new security-selection or review-set diagnostic code is introduced, update the canonical definition, snapshot/severity map, remediation table, recovery tests, and retirement assertions.
  - depends_on: [t-04, t-05, t-07]
  - target_files: [internal/model/reason_code.go, internal/model/reason_code_contract_test.go, internal/model/recovery.go, internal/model/recovery_test.go, internal/engine/progression/authority.go, internal/engine/progression/authority_test.go]
  - task_kind: test
  - covers: [REQ-006, REQ-007, REQ-010]
  - acceptance: `go test ./internal/model ./internal/engine/progression -run 'TestCanonicalReason|Test.*Recovery|Test.*Reason|Test.*Context' -count=1` produces no `unknown_reason_code` downgrade.

- [x] `t-09` Refresh docs and durable context for the variable review set: update workflow/design docs and the stale codebase-map sections so they describe the mandatory trio, optional security selector, shared `stage=review` token, skill-keyed R2 lattice, selected-set chain order, fail-closed recovery, and structural-not-cryptographic trust tier.
  - depends_on: [t-03, t-04, t-05, t-06]
  - target_files: [docs/design.md, docs/workflow.md, artifacts/codebase/ARCHITECTURE.md, artifacts/codebase/STRUCTURE.md, artifacts/codebase/TESTING.md, artifacts/codebase/CONCERNS.md]
  - task_kind: doc
  - covers: [REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-010]
  - acceptance: docs and codebase map no longer describe fixed-pair S3 review as the final scope and consistently document the selected review set.

- [x] `t-10` Prove the full review-SET flow through focused tests, full tests, formatting/lint, and governed dogfood fixtures: update lifecycle fixtures to include independent-review and selected security records with distinct `stage=review` handles, keep plan-audit `plan_origin`/`audit_origin` evidence distinct, verify target files cover the current diff, and record fresh execution evidence after the implementation waves complete.
  - depends_on: [t-00, t-01, t-02, t-03, t-04, t-05, t-06, t-07, t-08, t-09]
  - target_files: [cmd/lifecycle_commands_test.go, internal/engine/progression/advance_governed_test.go, internal/engine/progression/authority_test.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/skill_resolution.go, internal/engine/progression/skill_resolution_test.go, internal/toolgen/toolgen_test.go, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates_test.go, artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/wave-orchestration-notes.md, artifacts/changes/feat-governance-host-native-subagent-enforced-cross-stage-in/verification/plan-audit-notes.md]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010]
  - acceptance: `go test ./...`, `gofmt -s -l .`, and available lint pass; governed validation reports a valid bundle and no stale fixed-pair plan remains.
