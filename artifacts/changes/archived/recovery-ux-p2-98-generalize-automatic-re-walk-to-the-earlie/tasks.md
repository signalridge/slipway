# Tasks
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Content-hash freshness + compile-safe restamp removal boundary: make `model.EvidenceFreshness` the only freshness comparator; remove the mtime `*ChangedAfterVerdict` family and the digest `run_version` checks + `SkillDigest.RunVersion`; rewrite the no-stored-digest branch and stamp-on-accept so a freshly-authored stage is fresh after its verdict (#90 root); remove the Tier-0 `RestampEvidenceDigest*` engine functions and the `evidence restamp` command registration/call in the same slice so wave 1 remains buildable.
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/progression/evidence_digests.go, internal/engine/progression/evidence_digests_test.go, internal/engine/progression/freshness_guard_test.go, internal/engine/progression/autopass.go, internal/model/evidence_digests.go, internal/model/evidence_digests_test.go, internal/state/evidence_digests_test.go, cmd/evidence.go, cmd/evidence_test.go, cmd/root.go]
  - task_kind: code
  - covers: [REQ-002, REQ-007]
  - evidence: verdict
  - acceptance: Freshness comparisons use content hashes only; freshness_guard has no ModTime allowlist; skill digests no longer store or compare `SkillDigest.RunVersion`, while `VerificationRecord.RunVersion`, `ExecutionSummary.RunSummaryVersion`, task evidence run binding, run-summary-bound review checks, and closeout reuse checks remain intact; `evidence task` still builds/works while `evidence restamp` is no longer registered or called.

- [x] `t-02` #89 hash split foundation: add `TaskPlanStructuralHash`/`TaskPlanScopeHash`/`TaskPlanSemanticHash` and projections in wave/parse; add `WavePlan.TasksPlanStructuralHash`/`TasksPlanScopeHash`/`EffectiveStructuralHash`; materialize all three hashes and `CurrentTasksPlanStructuralState`/`CurrentTasksPlanScopeState`.
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/wave/parse.go, internal/engine/wave/parse_test.go, internal/model/wave_execution.go, internal/state/wave_execution.go, internal/state/wave_execution_test.go]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: verdict
  - acceptance: Structural hash includes every task execution/evidence contract field except target_files (task_id, objective, wave, depends_on, task_kind, covers, evidence, acceptance, checkpoint_type), scope hash changes on target_files-only edits, and semantic hash remains available only for intentional non-structural compatibility/legacy comparisons; runtime task-evidence drift MUST NOT key on semantic hash when only target_files changed, and MUST stale/reopen when any non-target_files task contract field changes.

- [x] `t-03` Pivot preserves runtime task evidence (#96): replace the blanket `os.RemoveAll(ChangeDir)` in reroute/rescope with targeted removal that preserves `evidence/tasks/`; verify the TaskPID cleanup path does not remove task evidence.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/pivot_execution.go, cmd/pivot_execution_test.go]
  - task_kind: code
  - covers: [REQ-005]
  - evidence: verdict
  - acceptance: Pivot reroute/rescope preserves compatible runtime task evidence and still clears non-task runtime residue, including stale PID/session files.

- [x] `t-11` Early self-bootstrap checkpoint after the freshness/restamp boundary: immediately after t-01, run this change's own `go run . validate --json`, `go run . next --json --diagnostics`, and `go run . health --governance --json` using the current worktree binary as a black-box caller; record any non-actionable self-stale or guess-required state as an implementation blocker before any later recovery work continues.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [artifacts/changes/recovery-ux-p2-98-generalize-automatic-re-walk-to-the-earlie/verification/early-self-bootstrap.yaml]
  - task_kind: verification
  - covers: [REQ-002, REQ-008, REQ-014]
  - evidence: checklist
  - acceptance: The current worktree binary keeps this change's active governance state actionable after t-01, or a concrete implementation blocker is filed and fixed before t-04/t-05/t-06/t-07 continue; no source inspection, digest/timestamp edit, or command guessing is required to decide the next action.

- [x] `t-04` Generalized reopen primitive: NEW `stale_evidence_recovery.go` — canonical ordering from `action.WorkflowPath`+`planSubStepOrder`; `staleReopenTarget` (iterate registry, content-hash freshness); `staleReopenFromExecSummary`; `reopenToStaleStage` (clear target+downstream, preserve runtime task evidence); read-only `StaleEvidenceRecoveryAvailable`; input-builder coverage assertions for governed artifacts and downstream skill inputs; explicit handling/test rationale for S1 `validate`.
  - wave: 3
  - depends_on: [t-11]
  - target_files: [internal/engine/progression/stale_evidence_recovery.go, internal/engine/progression/stale_evidence_recovery_test.go, internal/engine/progression/evidence_digests.go, internal/engine/progression/evidence_digests_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-010]
  - evidence: verdict
  - acceptance: Earliest target computation skips future authorities, chooses the earliest stale reached authority, orders/justifies all S1 substeps, and fails tests when a governed artifact is missing from expected certified inputs.

- [x] `t-05` #89 freshness wiring: plan-audit tasks.md input keys on structural hash; `tasksPlanChangedSinceTaskEvidenceBlockers` and task-evidence `freshness_inputs.tasks_plan_hash` compare structural hash, not semantic hash; wave_sync structural-stale + S2 in-place scope re-materialize only for strict target_files-only edits; execution-summary structural state; health uses effective structural hash.
  - wave: 3
  - depends_on: [t-02, t-11]
  - target_files: [internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go, internal/state/execution_summary.go, internal/state/execution_summary_test.go, internal/state/health.go, internal/state/health_test.go, cmd/common_test.go, cmd/execution_summary_test_helper_test.go, cmd/health_test.go]
  - task_kind: code
  - covers: [REQ-004, REQ-006]
  - evidence: verdict
  - acceptance: S2 target_files-only edits rematerialize wave-plan in place without emitting `tasks_plan_changed_since_task_evidence` for compatible runtime task evidence; changes to task_id, objective, wave, depends_on, task_kind, covers, evidence, acceptance, or checkpoint_type are structural and produce stale planning recovery instead of preserving runtime task evidence or creating a persistent S2 dead-end.

- [x] `t-06` Recovery vocabulary and repair reroute cleanup: route `repair` stale-digest drift and `required_skill_stale` recovery summaries to `slipway run`; remove restamp/Tier wording from recovery model tests and repair tests; add the no-bypass fixtures for guardrail-domain stale evidence.
  - wave: 3
  - depends_on: [t-11]
  - target_files: [cmd/repair.go, cmd/repair_test.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-007, REQ-009]
  - evidence: verdict
  - acceptance: Repair stale-digest guidance points to `slipway run`; recovery JSON no longer recommends restamp/Tier paths; guardrail-domain stale evidence has no restamp, attestation, or force-close bypass.

- [x] `t-07` Wire the unified reopen into `AdvanceGoverned` for all states at the two trigger points; delete `beginStalePlanningRecovery`, `stalePlanningRecoveryNeededForPlanAuditDigest`, and `stale_planning_recovery.go`; add first-class S0 intake clear-and-rerun (#90); add S2 internal stale handling that rebuilds compatible generated evidence or reopens earlier without evidence-loss rescope.
  - wave: 4
  - depends_on: [t-04]
  - target_files: [internal/engine/progression/advance_governed.go, internal/engine/progression/advance_intake.go, internal/engine/progression/stale_planning_recovery.go, cmd/lifecycle_commands_test.go, cmd/progression_next_test.go, cmd/cli_e2e_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003]
  - evidence: verdict
  - acceptance: `go run . run --json` recovers S0, S1, S2, S3, and S4 stale cases through the existing host skill flow; old stale-planning symbols/files are removed.

- [x] `t-08` #97 repair re-materialize: regenerate execution-summary/wave-plan when source inputs changed instead of returning the readable-but-stale file.
  - wave: 4
  - depends_on: [t-05, t-06]
  - target_files: [internal/state/execution_repair.go, internal/state/repair_test.go, cmd/repair.go, cmd/repair_test.go]
  - task_kind: code
  - covers: [REQ-006]
  - evidence: verdict
  - acceptance: Repair/run for stale readable wave-plan or execution-summary returns an executable rebuild/re-walk path and never tells the operator to manually edit generated evidence or pivot into task-evidence loss.

- [x] `t-09` Read-only/mutating parity + F1 + vocabulary: surface `StaleEvidenceRecoveryAvailable` in next/validate/status/CLIError with the computed target; F1 S1/audit `no_skill_required` → actionable `slipway run`; rename `stale_planning_recovery_available` → `stale_evidence_recovery_available`; update scope-contract recovery guidance strings; #89 S2 scope guidance.
  - wave: 5
  - depends_on: [t-07]
  - target_files: [cmd/next_skill_view.go, cmd/next.go, cmd/status.go, cmd/status_view_build.go, cmd/validate.go, cmd/errors.go, cmd/done.go, cmd/recovery_view_test.go, cmd/scope_contract_test.go, internal/model/recovery.go, internal/model/reason_code.go, internal/engine/progression/readiness.go]
  - task_kind: code
  - covers: [REQ-008, REQ-012]
  - evidence: verdict
  - acceptance: Read-only next/validate/status/CLIError and mutating run report the same root authority, recovery code, and public command for every replayed stale case; recovery JSON field shape remains documented and intentional `stale_planning_recovery_available` to `stale_evidence_recovery_available` vocabulary changes have contract tests.

- [x] `t-10` Agent docs, skills, and generated-surface alignment: keep `CLAUDE.md` and `AGENTS.md` as principle-only instruction surfaces; remove concrete Slipway command recipes and JSON examples while requiring black-box use of the latest current-worktree CLI; drop any `recover` route; ensure each stale case routes to the owning stage skill via `slipway run`; document the recovery JSON field shape and intentional reason-code rename; document commit-before-done diff-class staleness as review rerun only; regenerate all governed skills, Slipway workflow/command skills, and command references.
  - wave: 6
  - depends_on: [t-09]
  - target_files: [AGENTS.md, CLAUDE.md, README.md, docs/commands.md, docs/workflow.md, docs/operator-guide.md, internal/tmpl/templates/skills, internal/tmpl/templates/_partials, internal/toolgen]
  - task_kind: code
  - covers: [REQ-007, REQ-011, REQ-012, REQ-013, REQ-014, REQ-015]
  - evidence: artifact
  - acceptance: CLAUDE.md and AGENTS.md contain no concrete Slipway command tutorial, JSON classification example, or duplicated lifecycle mechanics; they require black-box latest-current-worktree Slipway use and immediate product repair for guess-required nodes. Generated skills/command references and repo docs match current CLI/recovery JSON behavior, document the intentional reason-code rename, contain no normal-path restamp/Tier recovery guidance, and consistently route stale inputs to the owning stage.

- [x] `t-12` Dead-end replay (dogfood) with `go run . run` alone — #90 (S0 intake), #89 (S2 structural reopen + scope-only in-place), #81 (S1 plan-audit), #96 (pivot preserves task evidence including TaskPID cleanup), #97 plus S2 repair gap (exec-summary/wave-plan regenerate); confirm read-only `next` matches mutating `run` target.
  - wave: 7
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06, t-07, t-08, t-09, t-10, t-11]
  - target_files: [cmd/lifecycle_commands_test.go]
  - task_kind: verification
  - covers: [REQ-001, REQ-004, REQ-005, REQ-006, REQ-008, REQ-014]
  - evidence: checklist
  - acceptance: Each named issue replay converges through the current worktree `go run . run`/normal stage rerun only, with no recover/restamp/manual digest/source-inspection/guessing step; #89 scope-only replay preserves compatible runtime task evidence only for strict target_files-only edits, and a replay that changes objective/dependencies/task_kind/evidence/acceptance proves S1/audit reopen rather than evidence preservation.

- [x] `t-13` Escape-hatch + guardrail audit: no `recover` command and no `evidence restamp` recovery path in `slipway --help` or generated skills/commands; no Tier 0/1/2 vocabulary; a guardrail-domain stale change is rerun/review-only (no restamp/force-close bypass).
  - wave: 7
  - depends_on: [t-01, t-04, t-06, t-07, t-09, t-10, t-11]
  - target_files: [cmd/evidence_test.go, AGENTS.md, CLAUDE.md, internal/tmpl/templates/skills, internal/tmpl/templates/_partials]
  - task_kind: verification
  - covers: [REQ-007, REQ-009, REQ-011, REQ-012, REQ-013, REQ-015]
  - evidence: checklist
  - acceptance: CLI help, generated surfaces, README/docs, CLAUDE.md, AGENTS.md, and recovery JSON have no user-facing recover/restamp/Tier taxonomy; agent instruction surfaces do not duplicate concrete command recipes; sensitive-domain stale evidence always routes to rerun/review; public-contract review expectations are visible for S3 (`layer:R3=pass`, `layer:IR3=pass`).

- [x] `t-14` Final proof stack and contract closeout gate: run `go build ./...`, `go vet ./...`, `go test ./...`, `go test ./internal/toolgen/...`, `go run . init --refresh --tools all` followed by a recorded zero-drift check such as `git diff --exit-code`, black-box lifecycle self-checks using the current worktree `go run .`, `freshness_guard` zero ModTime paths, `go run . validate --json`, and `go run . health --governance --json`; confirm S3 review evidence includes external-contract tokens before final closeout.
  - wave: 8
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06, t-07, t-08, t-09, t-10, t-11, t-12, t-13]
  - target_files: [internal/engine/progression/freshness_guard_test.go, internal/toolgen, AGENTS.md, CLAUDE.md, artifacts/changes/recovery-ux-p2-98-generalize-automatic-re-walk-to-the-earlie/verification/final-proof.yaml]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011, REQ-012, REQ-013, REQ-014, REQ-015]
  - evidence: checklist
  - acceptance: Full proof commands pass under the current worktree binary, `go run . init --refresh --tools all` plus the recorded drift check leaves zero project-visible generated-surface drift, CLAUDE.md, AGENTS.md, and all governed/Slipway skills match current CLI behavior without command-recipe drift, this change's self-governance state stays actionable without manual digest/timestamp edits or guessing, and external API contract review evidence is present before closeout.
