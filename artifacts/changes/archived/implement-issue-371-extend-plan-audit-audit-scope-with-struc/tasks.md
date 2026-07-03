# Tasks

## Task List

- [x] `t-01` Add the pure model parser, validation result model, reason codes, recovery mappings, and parser/recovery tests for plan-dimension attestations.
  - depends_on: []
  - target_files: [internal/model/plan_dimension_attestation.go, internal/model/plan_dimension_attestation_test.go, internal/model/reason_code.go, internal/model/reason_code_contract_test.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-005, REQ-008]

- [x] `t-02` Enforce required `decision_soundness` and `consistency` attestations in the S1 plan gate, including light-preset advisory behavior and the past-S1 no-rewalk guard.
  - depends_on: [t-01]
  - target_files: [internal/engine/progression/advance_governed.go, internal/engine/progression/advance_governed_test.go, cmd/next_plan_audit_handoff_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-005, REQ-008]

- [x] `t-03` Enforce review-owned required plan-dimension attestations for selected S3 `spec-compliance-review` evidence.
  - depends_on: [t-01]
  - target_files: [internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, cmd/progression_next_test.go, cmd/review_test.go, cmd/governance_gate_consistency_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-005, REQ-008]

- [x] `t-04` Add `slipway evidence skill` early validation for passing `plan-audit` and selected S3 `spec-compliance-review` records while preserving failed-record behavior.
  - depends_on: [t-01]
  - target_files: [cmd/evidence.go, cmd/evidence_skill_test.go, cmd/evidence_task_test.go, cmd/lifecycle_commands_test.go, cmd/evidence_test.go]
  - task_kind: code
  - covers: [REQ-004, REQ-005, REQ-008]

- [x] `t-05` Add the deterministic unknown prose `REQ-*` consistency check without expanding into unsafe semantic inference.
  - depends_on: [t-01]
  - target_files: [internal/engine/progression/validation.go, internal/engine/progression/validation_test.go]
  - task_kind: code
  - covers: [REQ-006, REQ-008]

- [x] `t-06` Update plan-audit and spec-compliance-review templates, add the consistency sidecar, refresh generated surfaces, and pin generation contracts.
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/skills/plan-audit/HOST_SKILL.md, internal/tmpl/templates/skills/plan-audit/references/consistency-audit.md, internal/tmpl/templates/skills/spec-compliance-review/HOST_SKILL.md.tmpl, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest_test.go, internal/toolgen/testdata/skill_tree_inventory.codex.golden]
  - task_kind: doc
  - covers: [REQ-007, REQ-008]

- [x] `t-07` Run focused and repo-level verification, record task and wave evidence, and repair any failures exposed by the governed gates.
  - depends_on: [t-02, t-03, t-04, t-05, t-06]
  - target_files: [artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/tasks.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/plan-audit.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/wave-orchestration.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/wave-orchestration-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/execution-summary.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/spec-compliance-review.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/spec-compliance-review-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/code-quality-review.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/code-quality-review-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/independent-review.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/independent-review-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/security-review.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/security-review-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/ship-verification.yaml, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/ship-verification-notes.md, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-01-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-02-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-03-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-04-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-05-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-06-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/verification/t-07-result.json, artifacts/changes/implement-issue-371-extend-plan-audit-audit-scope-with-struc/assurance.md]
  - task_kind: verification
  - covers: [REQ-008]
