# Tasks

## Task List

- [ ] `t-01` Remove test-held wrappers and no-consumer internal API
  - depends_on: []
  - target_files: [cmd/common.go, cmd/status.go, cmd/status_view_build.go, cmd/validate.go, cmd/next_skill_view.go, cmd/common_test.go, cmd/status_view_build_test.go, cmd/status_timeline_test.go, cmd/stats_test.go, cmd/validate_requirements_contract_test.go, cmd/validate_artifact_gate_test.go, cmd/scope_contract_test.go, cmd/new_test.go, cmd/governance_gate_consistency_test.go, cmd/status_context_repair_test.go, cmd/progression_next_test.go, cmd/root_help_test.go, cmd/abort_test.go, internal/engine/progression/readiness.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go, internal/engine/progression/wave_boundary_evidence_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/worktree_binding_test.go, internal/state/handoff.go, internal/state/handoff_test.go, internal/state/execution_repair.go, internal/state/repair_test.go, internal/state/lifecycle_event.go, internal/state/lifecycle_event_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/state/execution_summary_test.go, internal/fsutil/transaction.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/engine/progression/advance_transaction_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: targeted wrappers/helpers are removed or tests are redirected to current production helpers; `ResumeWaveIndexFromTaskEvidence` is removed/inlined or has current-worktree evidence recorded in t-01 task evidence proving it remains live; focused command/state/progression/toolgen tests pass; focused unused lint no longer reports these findings.

- [ ] `t-02` Remove dead model/state/capability fields and methods
  - depends_on: []
  - target_files: [internal/model/types.go, internal/model/phase.go, internal/model/execution_summary.go, internal/model/execution_summary_test.go, internal/engine/capability/resolver.go, internal/state/repair.go, internal/state/health_test.go, internal/engine/skill/registry_loader.go, cmd/tool_github.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: confirmed no-writer/no-reader fields and no-consumer methods are removed; stale tests or snapshots are updated to live behavior; focused model/state/capability tests pass.

- [ ] `t-03` Remove inert retired-feature and narrow compatibility wiring
  - depends_on: [t-01, t-02]
  - target_files: [internal/engine/skill/skill.go, internal/engine/skill/skill_test.go, internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, internal/engine/progression/validation.go, internal/engine/progression/evidence.go, internal/engine/progression/evidence_repair.go, internal/engine/progression/advance_governed.go, cmd/evidence.go, cmd/evidence_skill_test.go, cmd/next_skill_view.go, cmd/stats.go, cmd/stats_test.go, cmd/status_view_build.go, cmd/status_view_build_test.go, cmd/status_timeline_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/health.go, internal/state/health_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/model/types.go, internal/model/change.go, internal/model/context_attestation.go, internal/model/context_attestation_test.go, internal/model/reason_code.go, internal/model/reason_code_contract_test.go, cmd/health_test.go, cmd/repair.go, cmd/repair_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003]
  - acceptance: CloseoutConditional and dead closeout-required threading are removed without weakening ship-verification; retired state canonicalization is removed as an explicit behavior change across the `WorkflowState` definition, change-load normalization, and status timeline rendering in the same task; legacy handoff hygiene and obsolete shims are removed; retired context-origin rejection remains covered; focused progression/skill/evidence/state/status tests pass.

- [ ] `t-04` Remove no-longer-emitted reason codes and remediations
  - depends_on: []
  - target_files: [internal/model/reason_code.go, internal/model/recovery.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go, cmd/recovery_view_test.go, cmd/evidence_skill_test.go, cmd/status_json_test.go, internal/engine/progression/wave_sync_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - acceptance: no-longer-emitted reason codes are removed from catalog, remediation, and frozen expectations; fabricated stale-code tests are rewritten or removed; focused reason-code tests pass.

- [ ] `t-05` Resolve lint-confirmed assignment, parameter, and duplicate-wiring cleanup
  - depends_on: [t-01, t-02]
  - target_files: [cmd/fix.go, cmd/fix_test.go, cmd/freshness_diagnostics.go, cmd/next_skill_view.go, cmd/progression_next_test.go, cmd/next_wave_plan.go, cmd/tool_github.go, internal/coverage/coverage.go, internal/engine/progression/authority.go, internal/engine/progression/advance_governed.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/evidence_digests_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/delete.go, internal/state/delete_test.go, internal/perfbaseline/cmd/state-read-baseline/main.go]
  - task_kind: code
  - covers: [REQ-001, REQ-005]
  - acceptance: confirmed unparam/staticcheck/ineffassign/wastedassign findings are removed; identical duplicate wiring is consolidated only where covered; focused cleanup lint commands no longer report targeted findings.

- [ ] `t-06` Remove dead config, state counters, and public no-op command flags
  - depends_on: [t-02, t-03]
  - target_files: [internal/model/config.go, internal/model/config_test.go, internal/model/config_catalog.go, internal/model/config_catalog_test.go, internal/engine/progression/validation.go, internal/engine/progression/validation_test.go, internal/engine/artifact/requirements_contract.go, internal/engine/artifact/requirements_contract_test.go, internal/model/change.go, cmd/config_test.go, cmd/review.go, cmd/review_test.go, cmd/done.go, cmd/lifecycle_commands_test.go, cmd/validate.go, cmd/validate_readonly_test.go, cmd/root_help_test.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest_test.go, docs/reference/commands.md, docs/SURFACE-MANIFEST.json]
  - task_kind: code
  - covers: [REQ-001, REQ-007]
  - acceptance: validation no-op flags, write-only review drift counters, and no-op done/validate --json flags are removed; live core/expanded/custom artifact schema and command behavior remain tested; config catalog/config command output and generated surface manifest are synchronized.

- [ ] `t-07` Consolidate remaining confirmed redundancy surfaces
  - depends_on: [t-03, t-04, t-05, t-06]
  - target_files: [cmd/freshness_diagnostics.go, cmd/status_view_build.go, cmd/status.go, cmd/status_render.go, cmd/status_json_test.go, cmd/status_view_build_test.go, cmd/status_render_test.go, cmd/common.go, cmd/common_test.go, cmd/validate.go, cmd/validate_readonly_test.go, cmd/next.go, cmd/next_handoff.go, cmd/progression_next_test.go, cmd/done.go, cmd/lifecycle_commands_test.go, cmd/evidence.go, cmd/evidence_skill_test.go, cmd/tool_github.go, cmd/tool_github_test.go, cmd/stats.go, cmd/stats_test.go, internal/engine/progression/evidence_repair.go, internal/engine/progression/evidence_repair_test.go, internal/engine/progression/readiness.go, internal/engine/progression/readiness_test.go, internal/engine/artifact/decision_contract.go, internal/engine/artifact/decision_contract_test.go, internal/engine/artifact/requirements_contract.go, internal/engine/artifact/requirements_contract_test.go, internal/engine/artifact/tasks_contract.go, internal/engine/artifact/tasks_contract_test.go, internal/model/reason_code.go, internal/model/recovery.go, internal/model/model_test.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go, internal/state/execution_summary.go, internal/state/execution_summary_test.go, internal/state/evidence_digests.go, internal/state/evidence_digests_test.go, internal/state/wave_execution.go, internal/state/wave_execution_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/engine/governance/test_helpers_test.go, internal/engine/governance/runtime_actions_test.go, internal/engine/progression/test_helpers_test.go, internal/engine/progression/advance_governed_test.go, internal/fsutil/repo_root.go, internal/fsutil/repo_root_test.go, internal/toolgen/cmd/gen-surface-manifest/main.go, internal/coverage/cmd/covergate/main.go, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl, internal/tmpl/templates/skills/security-review/SKILL.md.tmpl, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/_partials/review-disk-handoff.tmpl, internal/tmpl/templates/skills/_partials/review-record-verification.tmpl, internal/tmpl/templates_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-07.json]
  - task_kind: code
  - covers: [REQ-001, REQ-005, REQ-008]
  - acceptance: task evidence records a per-candidate disposition for every medium-priority redundancy candidate from the pasted reports before final assurance aggregation; command route/freshness helpers explicitly address `statusRoute` vs route-kind overlap and `EvidenceFreshness` vs `ExecutionEvidenceFreshness` synchronization; `cmd/tool_github` pagination, check-run envelope, and status extraction duplication are consolidated or proven still-live with current evidence in the t-07 task result; stale evidence repair predicates, artifact contract helper boilerplate, strict cache loaders, load-error wrappers, `blockerRemediations` vs `canonicalReasonDefinitions` drift, S3 review template text, verification test helpers, and tiny-binary root discovery no longer maintain duplicated logic; focused command/state/artifact/template tests pass.

- [ ] `t-08` Record final verification evidence and assurance
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06, t-07]
  - target_files: [artifacts/changes/cleanup-stale-code-surfaces/assurance.md, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-08.json]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008]
  - acceptance: final verification records current full test/lint/focused lint/surface-manifest/lifecycle outputs; assurance maps requirements to evidence and residual risks; validate and next show readiness.
