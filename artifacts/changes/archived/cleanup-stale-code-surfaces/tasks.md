# Tasks

## Task List

- [x] `t-01` Remove test-held wrappers and no-consumer internal API
  - depends_on: []
  - target_files: [cmd/common.go, cmd/status.go, cmd/status_view_build.go, cmd/validate.go, cmd/next_skill_view.go, cmd/common_test.go, cmd/status_view_build_test.go, cmd/status_timeline_test.go, cmd/stats_test.go, cmd/validate_requirements_contract_test.go, cmd/validate_artifact_gate_test.go, cmd/scope_contract_test.go, cmd/new_test.go, cmd/governance_gate_consistency_test.go, cmd/status_context_repair_test.go, cmd/progression_next_test.go, cmd/root_help_test.go, cmd/abort_test.go, internal/engine/progression/readiness.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go, internal/engine/progression/wave_boundary_evidence_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/worktree_binding_test.go, internal/state/handoff.go, internal/state/handoff_test.go, internal/state/execution_repair.go, internal/state/repair_test.go, internal/state/lifecycle_event.go, internal/state/lifecycle_event_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/state/execution_summary_test.go, internal/fsutil/transaction.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/engine/progression/advance_transaction_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-01.json]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: targeted wrappers/helpers are removed or tests are redirected to current production helpers; `ResumeWaveIndexFromTaskEvidence` is removed/inlined or has current-worktree evidence recorded in t-01 task evidence proving it remains live; focused command/state/progression/toolgen tests pass; focused unused lint no longer reports these findings; `task-result-t-01.json` records dispositions for C-001 and C-002.

- [x] `t-02` Remove dead model/state/capability fields and methods
  - depends_on: []
  - target_files: [internal/model/types.go, internal/model/phase.go, internal/model/execution_summary.go, internal/model/execution_summary_test.go, internal/engine/capability/resolver.go, internal/engine/progression/freshness_guard_test.go, internal/state/repair.go, internal/state/repair_test.go, internal/state/health_test.go, internal/engine/skill/registry_loader.go, cmd/tool_github.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-02.json]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: confirmed no-writer/no-reader fields and no-consumer methods are removed; stale tests or snapshots are updated to live behavior; focused model/state/capability tests pass; `task-result-t-02.json` records the disposition for C-003.

- [x] `t-03` Remove inert retired-feature and narrow compatibility wiring
  - depends_on: [t-01, t-02]
  - target_files: [internal/engine/skill/skill.go, internal/engine/skill/skill_test.go, internal/engine/progression/authority.go, internal/engine/progression/authority_test.go, internal/engine/progression/validation.go, internal/engine/progression/evidence.go, internal/engine/progression/evidence_test.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/evidence_repair.go, internal/engine/progression/advance_governed.go, internal/engine/progression/advance_intake.go, internal/engine/progression/readiness.go, cmd/evidence.go, cmd/evidence_skill_test.go, cmd/next.go, cmd/next_skill_view.go, cmd/stats.go, cmd/stats_test.go, cmd/status_view_build.go, cmd/status_view_build_test.go, cmd/status_timeline_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/health.go, internal/state/health_test.go, internal/state/verification.go, internal/state/verification_test.go, internal/model/types.go, internal/model/change.go, internal/model/model_test.go, internal/model/context_attestation.go, internal/model/context_attestation_test.go, internal/model/reason_code.go, internal/model/reason_code_contract_test.go, cmd/health_test.go, cmd/repair.go, cmd/repair_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-03.json]
  - task_kind: code
  - covers: [REQ-001, REQ-003]
  - acceptance: CloseoutConditional and dead closeout-required threading are removed without weakening ship-verification, after task evidence records the current registry/preset/template search proving no `closeout_conditional: true` setter exists; retired state canonicalization is removed as an explicit behavior change across the `WorkflowState` definition and change-load normalization, after task evidence records that source, tests, docs, and README surfaces no longer contain `S2_EXECUTE` or `S4_VERIFY` outside governed retirement notes; legacy handoff hygiene and obsolete shims are removed; retired context-origin rejection remains covered; focused progression/skill/evidence/state/status tests pass; `task-result-t-03.json` records dispositions for C-004 through C-006.

- [x] `t-04` Remove no-longer-emitted reason codes and remediations
  - depends_on: []
  - target_files: [internal/model/reason_code.go, internal/model/recovery.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go, cmd/recovery_view_test.go, cmd/evidence_skill_test.go, cmd/status_json_test.go, internal/engine/progression/wave_sync_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-04.json]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - acceptance: no-longer-emitted reason codes are removed from catalog, remediation, and frozen expectations; fabricated stale-code tests are rewritten or removed; focused reason-code tests pass; `task-result-t-04.json` records the disposition for C-007.

- [x] `t-05` Resolve lint-confirmed assignment, parameter, and duplicate-wiring cleanup
  - depends_on: [t-01, t-02]
  - target_files: [cmd/fix.go, cmd/fix_test.go, cmd/freshness_diagnostics.go, cmd/next_skill_view.go, cmd/progression_next_test.go, cmd/next_wave_plan.go, cmd/tool_github.go, internal/coverage/coverage.go, internal/engine/progression/authority.go, internal/engine/progression/advance_governed.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/evidence_digests_test.go, internal/state/store.go, internal/state/store_test.go, internal/state/lifecycle.go, internal/state/delete.go, internal/state/delete_test.go, internal/perfbaseline/cmd/state-read-baseline/main.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-05.json]
  - task_kind: code
  - covers: [REQ-001, REQ-005]
  - acceptance: confirmed unparam/staticcheck/ineffassign/wastedassign findings are removed; auto-skip evidence suppression is recorded as an intentional command-output behavior repair and covered by command-level `next`/`run`/`implement` tests; identical duplicate wiring is consolidated only where covered; focused cleanup lint commands no longer report targeted findings; `task-result-t-05.json` records dispositions for C-008 and C-009.

- [x] `t-06` Remove dead config, state counters, and public no-op command flags
  - depends_on: [t-02, t-03]
  - target_files: [internal/model/config.go, internal/model/config_test.go, internal/model/config_catalog.go, internal/model/config_catalog_test.go, internal/engine/progression/validation.go, internal/engine/progression/validation_test.go, internal/engine/artifact/requirements_contract.go, internal/engine/artifact/requirements_contract_test.go, internal/model/change.go, cmd/active_change_resolution_test.go, cmd/cli_e2e_test.go, cmd/config_test.go, cmd/delete_test.go, cmd/governance_readiness_error_test.go, cmd/hydrate_flag_test.go, cmd/review.go, cmd/review_test.go, cmd/done.go, cmd/run.go, cmd/lifecycle_commands_test.go, cmd/template_flag_contract_test.go, cmd/validate.go, cmd/validate_readonly_test.go, cmd/worktree_preflight_test.go, cmd/root_help_test.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest_test.go, internal/perfbaseline/baseline_test.go, internal/perfbaseline/cmd/state-read-baseline/main.go, internal/engine/artifact/manager_test.go, internal/tmpl/templates/_partials/command-done-body.tmpl, internal/tmpl/templates/_partials/command-preset-body.tmpl, internal/tmpl/templates/_partials/command-validate-body.tmpl, internal/tmpl/templates/artifacts/assurance.md, internal/tmpl/templates/skills/ship-verification/SKILL.md.tmpl, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, README.md, README.ja.md, README.zh.md, docs/commands.md, docs/contributing.md, docs/explanation/workflow.md, docs/how-to/recover-and-troubleshoot.md, docs/operator-guide.md, docs/real-world-scenarios.md, docs/start-here.md, docs/tutorials/first-governed-change.md, docs/tutorials/onboarding-existing-codebase.md, docs/workflow.md, docs/ja/commands.md, docs/ja/contributing.md, docs/ja/explanation/workflow.md, docs/ja/how-to/recover-and-troubleshoot.md, docs/ja/operator-guide.md, docs/ja/real-world-scenarios.md, docs/ja/reference/commands.md, docs/ja/start-here.md, docs/ja/tutorials/first-governed-change.md, docs/ja/tutorials/onboarding-existing-codebase.md, docs/ja/workflow.md, docs/zh/commands.md, docs/zh/contributing.md, docs/zh/explanation/workflow.md, docs/zh/how-to/recover-and-troubleshoot.md, docs/zh/operator-guide.md, docs/zh/real-world-scenarios.md, docs/zh/reference/commands.md, docs/zh/start-here.md, docs/zh/tutorials/first-governed-change.md, docs/zh/tutorials/onboarding-existing-codebase.md, docs/zh/workflow.md, docs/reference/commands.md, docs/zh/reference/commands.md, docs/SURFACE-MANIFEST.json, state-read-performance-baseline.json, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-06.json]
  - task_kind: code
  - covers: [REQ-001, REQ-007]
  - acceptance: validation no-op flags, write-only review drift counters, and no-op done/validate --json flags are removed from command code, tests, README, reference docs, and generated surface metadata; live core/expanded/custom artifact schema behavior, including `custom_artifacts`, remains tested and is not removed; config catalog/config command output and generated surface manifest are synchronized; task evidence records dispositions for C-010 through C-013.

- [x] `t-07` Consolidate command route and freshness wiring
  - depends_on: [t-03, t-04, t-05, t-06]
  - target_files: [cmd/freshness_diagnostics.go, cmd/status_view_build.go, cmd/status.go, cmd/status_render.go, cmd/status_json_test.go, cmd/status_view_build_test.go, cmd/status_render_test.go, cmd/common.go, cmd/common_test.go, cmd/validate.go, cmd/validate_readonly_test.go, cmd/next.go, cmd/next_handoff.go, cmd/progression_next_test.go, cmd/done.go, cmd/lifecycle_commands_test.go, cmd/evidence.go, cmd/evidence_skill_test.go, cmd/stats.go, cmd/stats_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-07.json]
  - task_kind: code
  - covers: [REQ-001, REQ-005, REQ-008]
  - acceptance: command route/freshness helpers explicitly address C-014, including `statusRoute` vs route-kind overlap and `EvidenceFreshness` vs `ExecutionEvidenceFreshness` synchronization; duplicated wiring is consolidated or proven still-live with current-worktree evidence; focused command JSON/text tests pass; `task-result-t-07.json` records a candidate disposition.

- [x] `t-08` Consolidate GitHub helper duplication
  - depends_on: [t-02, t-05]
  - target_files: [cmd/tool_github.go, cmd/tool_github_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-08.json]
  - task_kind: code
  - covers: [REQ-001, REQ-008]
  - acceptance: C-015 pagination, check-run envelope, and status extraction duplication in `cmd/tool_github` is consolidated or proven still-live with current evidence; `cmd/tool_github_test.go` is an intentional planned output if focused coverage is needed; GitHub helper output semantics remain covered; `task-result-t-08.json` records the candidate disposition.

- [x] `t-09` Consolidate engine, artifact, cache, and recovery helpers
  - depends_on: [t-03, t-04, t-06]
  - target_files: [internal/engine/progression/evidence_repair.go, internal/engine/progression/evidence_repair_test.go, internal/engine/progression/readiness.go, internal/engine/progression/readiness_test.go, internal/engine/artifact/decision_contract.go, internal/engine/artifact/decision_contract_test.go, internal/engine/artifact/requirements_contract.go, internal/engine/artifact/requirements_contract_test.go, internal/engine/artifact/tasks_contract.go, internal/engine/artifact/tasks_contract_test.go, internal/model/reason_code.go, internal/model/recovery.go, internal/model/model_test.go, internal/model/reason_code_contract_test.go, internal/model/recovery_test.go, internal/state/execution_summary.go, internal/state/execution_summary_test.go, internal/state/evidence_digests.go, internal/state/evidence_digests_test.go, internal/state/wave_execution.go, internal/state/wave_execution_test.go, internal/state/verification.go, internal/state/verification_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-09.json]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-008]
  - acceptance: C-016 through C-019 are each consolidated or proven still-live with current evidence; strict decoding, fail-closed recovery, artifact contract validation, and reason/remediation completeness are not weakened; focused progression/artifact/model/state tests pass; `task-result-t-09.json` records per-candidate dispositions.

- [x] `t-10` Extract S3 review template partials
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl, internal/tmpl/templates/skills/security-review/SKILL.md.tmpl, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/_partials/review-disk-handoff.tmpl, internal/tmpl/templates/skills/_partials/review-record-verification.tmpl, internal/tmpl/templates_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-10.json]
  - task_kind: code
  - covers: [REQ-001, REQ-008]
  - acceptance: C-020 repeated disk-handoff and record-verification template contracts are moved to shared partials or proven still-live; the two `_partials` files are intentional planned outputs if extraction is chosen; rendered template tests remain synchronized; `task-result-t-10.json` records the candidate disposition.

- [x] `t-11` Consolidate verification test helpers
  - depends_on: [t-03]
  - target_files: [internal/engine/governance/test_helpers_test.go, internal/engine/governance/runtime_actions_test.go, internal/engine/progression/test_helpers_test.go, internal/engine/progression/advance_governed_test.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-11.json]
  - task_kind: code
  - covers: [REQ-001, REQ-008]
  - acceptance: C-021 verification-writing helper duplication is consolidated or proven still-live without reducing behavior coverage; focused governance/progression tests pass; `task-result-t-11.json` records the candidate disposition.

- [x] `t-12` Share tiny-binary repository root discovery
  - depends_on: []
  - target_files: [internal/fsutil/repo_root.go, internal/fsutil/repo_root_test.go, internal/toolgen/cmd/gen-surface-manifest/main.go, internal/coverage/cmd/covergate/main.go, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-12.json]
  - task_kind: code
  - covers: [REQ-001, REQ-008]
  - acceptance: C-022 tiny command `findRepoRoot` duplication is replaced with a shared internal filesystem helper or proven still-live; `internal/fsutil/repo_root.go` and its test are intentional planned outputs if sharing is chosen; both tiny binaries keep current-root behavior; `task-result-t-12.json` records the candidate disposition.

- [x] `t-13` Record final verification evidence and assurance
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06, t-07, t-08, t-09, t-10, t-11, t-12]
  - target_files: [artifacts/changes/cleanup-stale-code-surfaces/assurance.md, artifacts/changes/cleanup-stale-code-surfaces/redundancy-candidates.md, artifacts/changes/cleanup-stale-code-surfaces/tasks.md, artifacts/changes/cleanup-stale-code-surfaces/verification/s3-review-repair-notes.md, artifacts/changes/cleanup-stale-code-surfaces/verification/task-result-t-13.json]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008]
  - acceptance: final verification records current full test/lint/focused lint/surface-manifest/lifecycle outputs; assurance maps requirements to task evidence, `redundancy-candidates.md`, and residual risks; assurance explicitly names the intentional BREAKING retirements for `done/validate --json` and `S2_EXECUTE`/`S4_VERIFY` normalization with current-worktree checks proving no source/test/doc/README surface keeps those retired workflow-state tokens; validate and next show readiness.
