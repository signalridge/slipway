# Intent

## Summary
Add engine-enforced fail-closed safety nets for shared-worktree wave parallelism. Engine governs evidence; host still executes (no engine spawn runtime). Changes: (C0) align dispatch token literal so WaveDispatchParallel value becomes parallel_subagents to match the wave-orchestration skill; (C1) post-run changed-file audit via exported wave.TargetCoversPath plus task_changed_file_scope_escape and parallel_wave_changed_file_overlap blockers; (C2) remove silent parallel inference in waveRunDispatchMode and add dispatch_mode_absent_on_started_parallel_wave fail-closed blocker; (C3) executor_agent engine validation adding executor_agent_missing for parallel_subagents waves; (C4) align generated wave-orchestration skill and docs; (C5) non-blocking plan-audit wave-narrowing advisories (broad_target_files, fully_serial_plan). Touches public validate/next/status JSON blocker-code set, WaveRun.dispatch_mode output value, and generated skill/docs.
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->
Rationale: changes machine-readable public surfaces (validate/next/status JSON blocker-code set, WaveRun.dispatch_mode output value) plus generated skill/docs; introduces fail-closed gates with a deliberate breaking change (C2). Blast radius spans engine/state/model/cmd/tmpl/toolgen with deterministic regression coverage required.

## Guardrail Domains
external_api_contracts

## In Scope
- `internal/model/wave_execution.go`: C0 change `WaveDispatchParallel` value `"parallel"`→`"parallel_subagents"` (Go const name unchanged); C3 add `ExecutorAgentHandlesFromVerification(record) map[int]map[string]string`.
- `internal/state/wave_execution.go`: C2 remove the two silent `WaveDispatchParallel` inference branches in `waveRunDispatchMode` (return `""` instead).
- `internal/engine/wave/wave.go`: C1 export `TargetCoversPath(targets []string, file string) bool` wrapping existing `normalizeTargetFileForConflict`/`targetFileContains`/`targetPatternMatches`; C5 add `AnalyzeWaveNarrowingCauses(nodes) []string`.
- `internal/engine/progression/wave_sync.go`: add blockers `TaskChangedFileScopeEscapeBlockers`, `ParallelWaveChangedFileOverlapBlockers` (C1), `DispatchEvidenceBlockers` (C2), `ExecutorAgentBlockers` (C3); wire all into `evaluateGovernedWaveExecution` at the `incompleteBlockers` assembly (~:173-180) under the existing `len(planDriftBlockers)==0` guard.
- `internal/model/reason_code.go`: define new blocker codes `task_changed_file_scope_escape`, `parallel_wave_changed_file_overlap`, `dispatch_mode_absent_on_started_parallel_wave`, `executor_agent_missing` in `canonicalReasonDefinitions` with a comment block.
- `cmd/next.go` + `cmd/next_wave_plan.go`: C5 add view-only `Advisories []string` (omitempty) to the wave-plan view, populated on both derived and from-model paths; excluded from `wave-plan.yaml`/`tasks_plan_hash`.
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` + `references/executor-dispatch-reference.md`: C4 declare the four engine-enforced blockers and the accurate-`target_files`/exhaustive-`changed_files` safety model; regenerate host surfaces via toolgen.
- Tests: new unit coverage for each blocker + analyzer; rewrite the dispatch-inference state/model tests that C2 invalidates.

## Out of Scope
- Per-executor worktree isolation / worktree proof tokens (user deliberately rejects; isolation stays change-level).
- gsd-core-style post-merge integrated build+test gate (deferred to a follow-up issue).
- Rewriting `PlanWaves` for globally-widest packing (keep greedy; add read-only helpers + advisories only).
- A third parallelization profile (adaptive/prefer); `forced`(default)/`off` stay as-is.
- Any in-engine spawn runtime; making C5 advisories blocking; a dual-literal transition for the dispatch token.
- The unrelated active change `explain-domain-review-mapping` (#203) — not touched.

## Constraints
- Treat the current worktree's Slipway CLI/behavior as authority; dogfood with a worktree-built binary (installed 0.22.1 lags and lacks these gates).
- Do not hand-edit engine-owned verification freshness state; record evidence via `slipway evidence`.
- Reuse existing conflict predicates; do not author a second "coverage" implementation (C1 must match planner conflict semantics).
- Keep C4 wording clear of the literals `"engine rejects"`/`"engine-level rejection"` (guarded by `wave_isolation_content_test.go:32-33`).
- New blockers surface only through `ExecutionSummary.OpenBlockers`; no new persistence or JSON pipeline.

## Acceptance Signals
- `go build ./... && go vet ./... && go test ./... && gofmt -l` all green / no output.
- Regression green: `internal/engine/wave/wave_test.go`, `cmd/next_wave_plan_test.go`, `internal/toolgen`, `internal/tmpl/wave_isolation_content_test.go`, `thin_host_content_test.go`.
- New tests prove: scope-escape fires on an out-of-target changed file but passes a file covered by a directory/glob target (proving it routes through `TargetCoversPath`); overlap fires only on parallel waves and not across sequential waves sharing a file; a started parallel wave missing `dispatch_mode` is blocked while `degraded_sequential` passes; a `parallel_subagents` wave missing an `executor_agent` handle is blocked while `degraded_sequential` requires none; `AnalyzeWaveNarrowingCauses` reports directory/glob targets and a linear chain but passes an honest single-task wave and pure file-conflict serialization; both preview(mutate=false) and mutate=true paths surface the new blockers in `OpenBlockers`.
- CLI dogfood: a worktree-built binary running `slipway next --json` surfaces `wave_plan.advisories` and the new `OpenBlockers`.

## Open Questions
None.

## Deferred Ideas
- gsd-core post-merge integrated build+test gate (cross-task breakage detection) as a separate change.

## Approved Summary
Engine-enforced fail-closed safety nets for shared-worktree wave parallelism (C0–C5). The engine signals, records, and gates; the host (Claude Code subagents) still executes — no in-engine spawn runtime.

- C0: `WaveDispatchParallel` value → `parallel_subagents` (matches the wave-orchestration skill's emitted token).
- C1: post-run changed-file audit via exported `wave.TargetCoversPath` → blockers `task_changed_file_scope_escape`, `parallel_wave_changed_file_overlap`.
- C2: remove silent parallel inference in `waveRunDispatchMode`; add fail-closed `dispatch_mode_absent_on_started_parallel_wave`.
- C3: add `executor_agent_missing` for `parallel_subagents` waves.
- C4: align the generated wave-orchestration skill + executor-dispatch reference.
- C5: non-blocking plan-audit advisories `broad_target_files`, `fully_serial_plan`.

In scope: engine/state/model/cmd/tmpl plus their tests, including the reason-code taxonomy snapshot (`internal/model/reason_code_contract_test.go`, bumped by every new reason code in t-03/t-04/t-05) and the lifecycle fixture updates needed by the new C2 dispatch-evidence gate (`cmd/lifecycle_commands_test.go` now records `degraded_sequential` for single-threaded synthetic fixtures).

Out of scope: per-executor worktree isolation, in-engine spawn runtime, gsd-style post-merge integrated build gate, a third parallelization profile.

Primary acceptance: `go build ./... && go vet ./... && go test ./... && gofmt -l` all green; new tests prove each blocker fires and passes correctly; a worktree-built `slipway next --json` surfaces `wave_plan.advisories` and the new `OpenBlockers`.
