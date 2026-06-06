# Research

## Research Findings

### Architecture
- Current primitive `beginStalePlanningRecovery` (`internal/engine/progression/advance_governed.go:459-560`): clears plan-audit.yaml(+digest), wave-plan.yaml, execution-summary.yaml, and the 4 downstream review/goal/closeout records(+digests); **preserves** wave-orchestration.yaml(+digest) and runtime task evidence under `ChangeDir`; reopens via `TransitionTo(S1)`+`AdvancePlanSubStep(Audit)`. Gated to S3/S4 by `stalePlanningRecoveryState` (`internal/engine/progression/stale_planning_recovery.go:37-39`); target hard-coded `StalePlanningRecoveryTarget="S1_PLAN/audit"` (:10).
- Two triggers in `AdvanceGoverned`: digest trigger `stalePlanningRecoveryNeededForPlanAuditDigest` at `advance_governed.go:68-72` (before bundle check); exec-summary trigger `stalePlanningRecoveryIssueAvailable` at `:106-108` (after wave sync + exec-summary load `:102-105`). Skill eval/stamp at `:141-176`; gates at `:222-292`.
- Registry `internal/engine/skill/skill.go:23-91` (`defaultGovernanceRegistry`), accessed via `LoadGovernanceRegistry` (`registry_loader.go:41`) and `LookupDefinitionInRegistry` (`skill.go:94`). Go map is authority; SKILL.md cannot override State/PlanSubStep. Skill → (State, PlanSubStep):
  - intake-clarification → (S0_INTAKE, —); research-orchestration → (S1_PLAN, research); plan-audit → (S1_PLAN, audit); wave-orchestration → (S2_EXECUTE, —); spec-compliance-review → (S3_REVIEW, —); code-quality-review → (S3_REVIEW, —); goal-verification → (S4_VERIFY, —); final-closeout → (S4_VERIFY, —).
- Transitions `internal/model/change_authority.go`: `TransitionTo` (:44-75) seeds entry substep for S0/S1, clears non-matching; `EnterIntake` (:16-26); `EnterPlanning` (:29-40); `AdvanceIntakeSubStep`/`AdvancePlanSubStep` set the substep directly. `ResetPivotExecutionResidue` (:161-176) clears evidence_refs/checkpoint/iterations but NOT on-disk runtime evidence.
- Canonical lifecycle order is the `action.WorkflowPath` slice (`internal/engine/action/workflow.go:10-18`: S0,S1,S2,S3,S4,Done) + `planSubStepOrder` (`advance_governed.go:443-447`: research,bundle,audit). The model `WorkflowState` is a plain string with no intrinsic numeric order — so ordering must derive from `action.WorkflowPath`, not a new rank table.

### Patterns
- Certified-input builders already hash **content** (`certifiedSkillInputDigest` dispatch `evidence_digests.go:36-85`; leaf hashes `model.ComputeInputHash`/`ComputeFileContentHash`). `model.EvidenceFreshness(stored, current)` (`internal/model/evidence_digests.go:100-125`) is **already pure content-hash** and should become the sole freshness authority.
- mtime freshness is only TWO sites: `evidence_digests.go:1100` and `:1114` inside `digestInputPathChangedAfterVerdict`, powering the `*ChangedAfterVerdict` family (`digestInputsChangedAfterVerdict` :982-989, `digestSelectedInputsChangedAfterVerdict` :991-1065, `digestInputChangedAfterVerdict` :1078). Removing mtime = remove this family and the branches that call it.
- `run_version` freshness is TWO sites: `evidence_digests.go:302-303` and `:442-443`; plus the `SkillDigest.RunVersion` field (`internal/model/evidence_digests.go:19`) and stamp `:110`. Other RunVersion uses (run-binding/reuse) are not freshness and stay.
- The #90 dead-end root: the no-stored-digest branch `evidence_digests.go:275-300` falls back to the mtime `digestInputsChangedAfterVerdict` path; must become a pure content-hash decision (stamp-on-accept so first run has a digest, and missing-entry-with-recorded-evidence reads fresh).
- Execution-summary (System B): tokens `StalePlanningEvidenceBlockerToken`/`StaleExecutionEvidenceBlockerToken` (`internal/state/execution_summary.go:23-24`), `stalePlanningPairs` :551-591 compares `summary.TasksPlanHash` vs `CurrentTasksPlanState` (#89 → structural); `wave_sync.go:80-192` + `tasksPlanChangedSinceTaskEvidenceBlockers` :571-618.
- #89 reference swaps: `evidence_digests.go:602` (plan-audit tasks.md input → structural hash), `execution_summary.go:562`, `wave_sync.go:122-161`, `health.go:330`, `execution_repair.go:158`; new `wave.TaskPlanStructuralHash`/`TaskPlanScopeHash` in `internal/engine/wave/parse.go`.
- Newly confirmed #89 trap: current `TaskPlanSemanticHash` includes `target_files`
  (`internal/engine/wave/parse.go:81-105`) and current runtime task-evidence
  drift calls `tasksPlanChangedSinceTaskEvidenceBlockers(previousHash, tasks,
  currentHash)` with `state.CurrentTasksPlanState` (`wave_sync.go:126-132`,
  `:571-618`). Therefore t-05 must explicitly move task-evidence drift and
  per-task `freshness_inputs.tasks_plan_hash` to structural hash; otherwise
  target_files-only edits stale runtime task evidence and violate #89.

### Risks
- Remove mtime/run_version freshness — MED: under-block if a passing skill is never stamped → make `stampPassingSkillDigests` (`evidence_digests.go:332-415`) stamp unconditionally on accept and route the no-digest branch through `EvidenceFreshness`. The `freshness_guard_test.go` allowlist (:24-33) names the three mtime functions and must be emptied when they are deleted.
- Generalize reopen to all states — MED: a stale stage not yet reached must not trigger (guard `pos > curPos → skip`). Read-only `StaleEvidenceRecoveryAvailable` and the mutating reopen must share one ordering function or they diverge.
- Clear-on-reopen vs old preserve-wave-orchestration — DESIGN CHOICE (see Alternatives). The old primitive preserved wave-orchestration on S1 reopen; the generalized `skillsAtOrAfterStage(targetPos)` clears every skill at/after target, so an S1/audit reopen clears wave-orchestration too. Clearing is more correct (a re-walk regenerates wave evidence from the preserved task evidence; a re-planned bundle invalidates the old execution verdict), but it flips `cmd/lifecycle_commands_test.go:967` which asserts preservation.
- #96 pivot evidence loss — HIGH (confirmed bug): `cmd/pivot_execution.go:76` `os.RemoveAll(state.ChangeDir(...))` deletes runtime task evidence (`ChangeDir/evidence/tasks/`) and wave runs (`evidence/waves/`). Fix = targeted removal preserving `evidence/tasks/`, consistent with the reopen primitive.
- External API contract drift — HIGH: the change alters public CLI/JSON and
  generated host-tool surfaces (`next`/`validate`/`status`/CLIError recovery
  JSON, `repair` guidance, `evidence restamp` availability, README/docs,
  CLAUDE/AGENTS agent guidance, and generated skills). Classify the change as
  `external_api_contracts`, add recovery JSON/help/docs/generated-surface
  contract tests, and require S3 external-contract review tokens
  (`layer:R3=pass`, `layer:IR3=pass`).
- Compile-safety boundary — MED: `cmd/evidence.go` registers `evidence restamp`
  and calls `progression.RestampEvidenceDigestTier0`; deleting the engine
  function before the CLI caller would break an intermediate wave. Remove the
  engine function and CLI command registration/call in one compile-safe task,
  then handle remaining repair/recovery vocabulary separately.
- Self-bootstrap timing — MED: freshness semantics govern this change's own
  evidence. Run this change's `go run . validate/next/health` immediately after
  the compile-safe freshness/restamp boundary, not only at final verification.
- Same-wave self-bootstrap bypass — MED: wave-orchestration may dispatch tasks
  inside one wave in parallel. Put t-11 in its own wave immediately after t-01
  and make t-04/t-05/t-06 depend on t-11 so generalized recovery work cannot
  proceed before the black-box self-check.
- Agent/skill drift — MED: root agent instruction files and generated skills can
  teach stale command recipes after CLI behavior changes. `CLAUDE.md` and
  `AGENTS.md` must exist as principle-only surfaces, stop carrying concrete
  Slipway usage walkthroughs, require black-box latest-current-worktree
  self-dogfooding, and treat guess-required nodes as product/usability defects;
  all governed/Slipway skills must regenerate or update to match current CLI
  and recovery JSON vocabulary.
- Reversibility: high — evidence cleared on reopen is regenerated by re-running; no durable user data touched.

### Test Strategy
- Existing coverage: `internal/engine/progression/freshness_guard_test.go` (meta-test forbidding ModTime in freshness paths — allowlist must shrink to empty), `cmd/lifecycle_commands_test.go:967` (`TestRunStalePlanningEvidenceReopens…` — generalize + reconcile wave-orchestration assertion), `evidence_digests_test.go`, `wave_sync_test.go`, `internal/model/evidence_digests_test.go` (RunVersion removal), `cmd/evidence_test.go:18-162` (Tier-0 restamp — delete), `cmd/repair_test.go:1132` (restamp routing — rewrite), `cmd/recovery_view_test.go` + `internal/model/recovery_test.go` (recovery object — update token rename), `internal/engine/wave/parse_test.go` (#89 hashes).
- Infrastructure: reuse governed-change fixtures (`prepareStalePlanningRecoveryFixture`); add cases — S2 structural reopen, S0 intake stale (#90), code-changed-at-S4 reopens S3, pivot preserves task evidence (#96), exec-summary regenerate (#97).
- Verification approach per acceptance signal: unit (ordering/earliest-stale via canonical path; reopen clears-forward + preserves task evidence; content-hash freshness fresh-after-edit-then-reverdict; #89 task-evidence drift uses structural hash), e2e (replay each dead-end with current-worktree `go run . run` alone), governance backstop (guardrail-domain change must re-run/review, never restamp), external API contract tests (recovery JSON shape, help/command availability, docs/generated-surface wording, CLAUDE/AGENTS no command recipes, all-skills/CLI alignment, S3 `R3`/`IR3` review token expectation), freshness-guard (zero ModTime), and dependency-locked early self-bootstrap after the freshness/restamp boundary.

## Alternatives Considered
- **Approach A — single generalized reopen + content-hash freshness (recommended).** New `stale_evidence_recovery.go`: detect earliest stale authority by iterating the registry, order by `action.WorkflowPath`+`planSubStepOrder` (no rank tables), reopen to that (state,substep) at any state, clear it + all downstream (incl. wave-orchestration on S1 reopen), preserve runtime task evidence; wire both triggers, delete `beginStalePlanningRecovery`/`stalePlanningRecoveryNeededForPlanAuditDigest`/`stale_planning_recovery.go`. Convert freshness to content-hash by making `model.EvidenceFreshness` the sole authority and removing the mtime `*ChangedAfterVerdict` family + run_version checks + `SkillDigest.RunVersion`. Port #89 split, including structural hash for runtime task-evidence drift. Fix #96/#97. Remove the Tier-0 `evidence restamp` surface + repair routing with a compile-safe t-01 boundary. Rename `stale_planning_recovery_available`→`stale_evidence_recovery_available` with the computed target. Treat the public CLI/JSON/generated-surface fallout as `external_api_contracts` with contract tests, CLAUDE/AGENTS black-box instruction cleanup, all-skills alignment, and S3 `R3`/`IR3` review evidence. Tradeoffs: largest diff; flips the wave-orchestration-preservation test; fully meets #98.
- **Approach B — minimal extension.** Keep `beginStalePlanningRecovery`; widen its state gate and parametrize the target; keep mtime/run_version freshness and the Tier-0 restamp. Tradeoffs: smallest diff, but leaves the mtime fragility (#90 persists), keeps two staleness heal paths, and keeps the escape-hatch — does not satisfy the user's full-delivery decision.
- **Approach C — diff-driven affected-authority index.** Compute affected authorities from a git diff of inputs via an artifact→authority index, drive a recover surface. Tradeoffs: reintroduces the #85 machinery the issue removes; rejected.
- **Selected: A** (full delivery), with the deliberate sub-decision that an S1 reopen **clears** wave-orchestration evidence (correctness over the old preserve behavior). User confirmation of the wave-orchestration clear-vs-preserve choice is recorded in intent.md.

## Unknowns
- Resolved: mtime/run_version freshness call sites → exactly `evidence_digests.go:1100/1114` (mtime) and `:302-303/:442-443` (run_version); conversion centers on making `model.EvidenceFreshness` the sole authority.
- Resolved: canonical ordering source → `action.WorkflowPath` + `planSubStepOrder`; S3/S4 same-position skills are fine because reopen granularity is the whole state.
- Resolved: what exists on main to remove → only the Tier-0 `evidence restamp` surface (`cmd/evidence.go:55-130`, `RestampEvidenceDigestTier0`) + repair routing (`repair.go:657-661`); `slipway recover`/Tier-2/`internal/engine/recovery` were never merged.
- Resolved: clear-vs-preserve wave-orchestration decision on S1 reopen — user selected uniform clear-forward for wave-orchestration evidence while preserving runtime task evidence.
- Resolved: guardrail classification — public CLI/JSON recovery behavior and
  generated host-tool surfaces are externally consumed contracts, so this is
  `external_api_contracts` despite operating on regenerable engine-owned
  evidence.
- Resolved: CLAUDE/AGENTS boundary — agent instruction files should not be
  Slipway command manuals. `CLAUDE.md` and `AGENTS.md` must be principle-only
  entry surfaces that require black-box current-worktree flow and treat
  guessing as a product defect while detailed command contracts remain in the
  CLI/generated surfaces.

## Assumptions
- Runtime task evidence is sufficient for wave-orchestration to re-derive wave-plan/execution-summary after a reopen — Evidence: `addWaveOrchestrationInputs` (`evidence_digests.go:869-918`) reads run_version+tasks from `ChangeDir`, preserved by the reopen.
- The reference patch ports cleanly (no main drift since 0.7.0) — Evidence: `git log 30f6fa5..HEAD` empty on core files.
- The external-contract burden is reviewable within this change because the
  affected surfaces are local CLI/help/docs/generated templates and JSON
  recovery rendering tests, not third-party network APIs.
- `AGENTS.md` was absent in the current target worktree. That absence is an
  avoidable agent-entry ambiguity for this repository, so the implementation
  should add a principle-only `AGENTS.md` companion to `CLAUDE.md` rather than
  relying on injected or remembered instructions.

## Canonical References
- `internal/engine/progression/advance_governed.go:459-560,68-72,106-108,443-447` (primitive + triggers + substep order)
- `internal/engine/progression/stale_planning_recovery.go:10,37-39` (S3/S4 gate + hard-coded target — to delete)
- `internal/engine/progression/evidence_digests.go:1091-1124,982-1078,275-320,332-415,154-216,602` (mtime family, no-digest branch, stamp, Tier-0 restamp, #89 swap)
- `internal/model/evidence_digests.go:100-125,19` (`EvidenceFreshness` content-hash; `SkillDigest.RunVersion`)
- `internal/engine/skill/skill.go:23-91` (registry) ; `internal/engine/action/workflow.go:10-18` (canonical order)
- `internal/model/change_authority.go:44-75` (transitions)
- `cmd/pivot_execution.go:76` (#96) ; `internal/state/execution_summary.go:23-24,551-591` (System B)
- `cmd/evidence.go:55-130` + `cmd/repair.go:657-661` (Tier-0 restamp surface + routing to remove)
- `internal/model/recovery.go:336,555-585` + `internal/model/reason_code.go:323` + `cmd/next_skill_view.go:182-197` (recovery object + token rename)
- `internal/engine/progression/freshness_guard_test.go:24-33` ; `cmd/lifecycle_commands_test.go:967`
- `CLAUDE.md`; `AGENTS.md`; `internal/tmpl/templates/skills`; `internal/tmpl/templates/_partials`; `internal/toolgen` (agent docs and generated skills/commands to keep aligned)
