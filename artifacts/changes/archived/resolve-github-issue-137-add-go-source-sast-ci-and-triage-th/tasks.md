# Tasks

## Task List

- [x] `t-01` Add gosec and CodeQL SAST jobs to the Security workflow.
  - wave: 1
  - depends_on: []
  - target_files: [.github/workflows/security.yaml]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - evidence: "workflow diff plus static inspection showing gosec and CodeQL jobs"
  - acceptance: "security.yaml contains a full-repo gosec SARIF job and a CodeQL Go analysis job without removing existing security jobs"

- [x] `t-02` Resolve all current HIGH full-repository gosec findings with fixes
  or local rule-specific suppressions.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/pivot_execution.go, cmd/pivot_execution_test.go, cmd/done.go, cmd/lifecycle_commands_test.go, internal/state/lifecycle.go, internal/engine/intake/intake.go, internal/engine/intake/intake_test.go, internal/engine/progression/autopass.go, internal/fsutil/atomic.go, internal/fsutil/no_symlink.go]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - evidence: "source diff and full gosec JSON report proving G101/G122/G703 are fixed or locally suppressed"
  - acceptance: "no unsuppressed HIGH gosec findings remain in full-repository scan"

- [x] `t-03` Resolve all current MEDIUM full-repository gosec permission/path
  findings with fixes or local rule-specific suppressions.
  - wave: 2
  - depends_on: [t-02]
  - target_files: [cmd/cancel.go, cmd/common.go, cmd/done.go, cmd/evidence.go, cmd/locks.go, cmd/new.go, cmd/next_skill.go, cmd/next_wave_plan.go, cmd/pivot_execution.go, internal/bootstrap/init.go, internal/engine/artifact/codebase_map.go, internal/engine/artifact/decision_contract.go, internal/engine/artifact/manager.go, internal/engine/artifact/requirements_contract.go, internal/engine/artifact/tasks_contract.go, internal/engine/governance/health.go, internal/engine/governance/policy_pack.go, internal/engine/governance/runtime_actions.go, internal/engine/governance/snapshot.go, internal/engine/governance/traceability.go, internal/engine/intake/intake.go, internal/engine/progression/advance_governed.go, internal/engine/progression/advance_intake.go, internal/engine/progression/autopass.go, internal/engine/progression/evidence.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/project_context.go, internal/engine/progression/readiness.go, internal/engine/progression/validation.go, internal/engine/progression/wave_sync.go, internal/engine/scopecontract/evaluate.go, internal/engine/skill/registry_loader.go, internal/engine/status/view.go, internal/fsutil/atomic.go, internal/fsutil/lock.go, internal/fsutil/root.go, internal/model/config.go, internal/model/evidence.go, internal/state/config_paths.go, internal/state/delete.go, internal/state/evidence_digests.go, internal/state/execution_repair.go, internal/state/execution_summary.go, internal/state/lifecycle.go, internal/state/lifecycle_event.go, internal/state/local_runtime_paths.go, internal/state/repair.go, internal/state/stats.go, internal/state/store.go, internal/state/verification.go, internal/state/wave_execution.go, internal/state/worktree.go, internal/state/worktree_binding.go, internal/toolgen/toolgen.go]
  - task_kind: code
  - covers: [REQ-003, REQ-005]
  - evidence: "source diff and full gosec JSON report proving G304/G301/G204/G306 are fixed or locally suppressed"
  - acceptance: "no unsuppressed MEDIUM gosec findings remain in full-repository scan"

- [x] `t-04` Re-run full-repository gosec JSON and SARIF scans and store fresh
  evidence under the governed verification directory.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: [artifacts/changes/resolve-github-issue-137-add-go-source-sast-ci-and-triage-th/verification/gosec-full.json, artifacts/changes/resolve-github-issue-137-add-go-source-sast-ci-and-triage-th/verification/gosec.sarif]
  - task_kind: verification
  - covers: [REQ-002, REQ-003, REQ-006]
  - evidence: "verification/gosec-full.json and verification/gosec.sarif"
  - acceptance: "full-repository gosec JSON and SARIF commands exit successfully with zero unsuppressed findings"

- [x] `t-05` Run full Go tests and workflow/static inspection evidence for the
  SAST configuration.
  - wave: 4
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: [artifacts/changes/resolve-github-issue-137-add-go-source-sast-ci-and-triage-th/verification/go-test.txt, artifacts/changes/resolve-github-issue-137-add-go-source-sast-ci-and-triage-th/verification/workflow-static-inspection.md, .github/workflows/security.yaml]
  - task_kind: verification
  - covers: [REQ-001, REQ-006]
  - evidence: "verification/go-test.txt and verification/workflow-static-inspection.md"
  - acceptance: "go test -count=1 ./... passes and workflow inspection confirms both SAST jobs and SARIF upload behavior"
