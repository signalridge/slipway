# Tasks

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add failing tests for content-digest freshness: checkbox-invariant tasks digest, mtime-invariant freshness, `assurance.md` included in `plan-audit` and `final-closeout`, real-edit→stale-names-artifact, guarded legacy backfill (safe backfill + drift-after-verdict refusal, including `wave-orchestration` runtime task evidence), and the #70/refreshed-verdict unchanged-tasks chain stays fresh while a real tasks change still goes stale.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/model/evidence_digests_test.go", "internal/state/evidence_digests_test.go", "internal/engine/progression/evidence_digests_test.go", "internal/engine/progression/evidence_test.go", "internal/state/wave_execution_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-008]

- [x] `t-02` Add failing tests for the certified input-set policy: non-ignored untracked reviewable files stale all diff-class reviews (`spec-compliance-review`, `code-quality-review`, `security-review`, `independent-review`), ignored/runtime evidence is excluded, and unrelated untracked files do not stale `goal-verification` unless recorded in the execution-summary changed/target set.
  - wave: 2
  - depends_on: []
  - target_files: ["internal/engine/progression/evidence_digests_test.go", "internal/engine/progression/readiness_test.go", "internal/engine/progression/readiness_optimization_test.go", "internal/engine/progression/authority_test.go"]
  - task_kind: test
  - covers: [REQ-005]

- [x] `t-03` Add failing tests for #67 S4 recovery routing on evidence task, verification_evidence_missing, and closeout_goal_verification_reuse_invalid.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/evidence_test.go", "internal/engine/progression/authority_test.go", "internal/engine/gate/gate_test.go"]
  - task_kind: test
  - covers: [REQ-006]

- [x] `t-04` Implement the digest primitive + engine-owned store: EvidenceDigests/SkillDigest + EvidenceFreshness; Save/LoadEvidenceDigests writing verification/evidence-digests.yaml; add it to the record-reader skip-list.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/model/evidence_digests.go", "internal/state/evidence_digests.go", "internal/state/verification.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-05` Stamp accepted digests at both mutating acceptance sites (AdvanceGoverned required-skill block + autopass authority paths) via a shared helper; compute per-skill input-sets (`plan-audit` includes `assurance.md`; `goal-verification` uses changed/target; `final-closeout` uses changed/target plus `assurance.md`; reviews use the explicit reviewable diff policy); apply guarded silent-backfill on legacy file-absent changes and emit `digest_backfilled_from_legacy_verdict`.
  - wave: 3
  - depends_on: [t-01, t-02, t-04]
  - target_files: ["internal/engine/progression/constants.go", "internal/engine/progression/evidence_digests.go", "internal/engine/progression/advance_governed.go", "internal/engine/progression/advance_intake.go", "internal/engine/progression/autopass.go", "internal/engine/progression/evidence.go", "internal/engine/progression/readiness.go", "internal/engine/progression/advance_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-005]

- [x] `t-06` Replace every steady-state time/mtime evidence-freshness branch with digest/semantic-hash comparison: authority.go closeout content freshness, execution_summary.go stale-planning/mtime baselines, context.go time path, execution_repair.go legacy fallback, and #70 wave-plan generated_at (MaterializeWavePlan no longer uses tasks.md mtime; generated_at is display/audit materialization time only; stale-planning chain keys on tasks_plan_hash); keep the wave-orchestration logical-CapturedAt, closeout proof-ordering, and legacy migration safety-gate carve-outs.
  - wave: 3
  - depends_on: [t-01, t-04]
  - target_files: ["internal/engine/progression/authority.go", "internal/engine/progression/wave_sync.go", "internal/engine/progression/wave_sync_test.go", "internal/state/execution_summary.go", "internal/state/execution_summary_test.go", "internal/engine/context/context.go", "internal/engine/context/context_test.go", "internal/state/execution_repair.go", "internal/state/health.go", "internal/state/wave_execution.go", "internal/model/execution_summary.go", "cmd/common_test.go", "cmd/evidence_task_test.go", "cmd/execution_summary_test_helper_test.go", "cmd/governance_gate_consistency_test.go", "cmd/progression_next_test.go", "cmd/repair_test.go", "cmd/review_test.go", "cmd/stats_test.go", "cmd/status_view_build_test.go", "cmd/validate_artifact_gate_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-008]

- [x] `t-07` #67: route S4 recovery remediation on evidence_task_wrong_state, verification_evidence_missing, and closeout_goal_verification_reuse_invalid to re-running goal-verification + final-closeout, naming the changed artifact when #66 digest diagnostics are available.
  - wave: 4
  - depends_on: [t-03, t-06]
  - target_files: ["cmd/evidence.go", "internal/engine/progression/authority.go", "internal/model/reason_code.go", "internal/model/model_test.go"]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-08` Add guard tests asserting no steady-state evidence-freshness path consults ModTime()/wall-clock now (excluding wave-orchestration logical CapturedAt, closeout proof-ordering, the explicit verdict safety-gate helpers, and enumerated display/logging sites); scan `execution_repair.go` as part of the production freshness surface and run focused regressions green.
  - wave: 5
  - depends_on: [t-05, t-06, t-07]
  - target_files: ["internal/engine/progression/freshness_guard_test.go", "internal/engine/progression/evidence_digests_test.go"]
  - task_kind: verification
  - covers: [REQ-003, REQ-007, REQ-008]

- [x] `t-09` Full proof: go build ./..., go test ./..., git diff --check, slipway validate --json fresh pass; update README/CLAUDE.md/docs for the evidence-digests surface, legacy migration gate, generated_at display boundary, and #59/#71 out-of-scope ledger.
  - wave: 6
  - depends_on: [t-05, t-06, t-07, t-08]
  - target_files: ["README.md", "CLAUDE.md", "docs/commands.md", "cmd/lifecycle_commands_test.go", "cmd/next_eval_fixture_test.go", "internal/state/worktree_binding_test.go", "artifacts/changes/fix-issues-59-66-67-together-under-one-thesis-governance-sig/assurance.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008]
