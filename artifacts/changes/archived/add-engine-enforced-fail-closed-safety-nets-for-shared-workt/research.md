# Research

## Alternatives Considered

### Architecture
- Affected modules / dependency chain: `tasks.md` → `internal/engine/wave/wave.go` (`PlanWaves` greedy depends_on layering + file-conflict bumping) → `MaterializeWavePlan` (stamps `WavePlanWave.Parallel`, `internal/model/wave_execution.go:26-35`) → `wave-plan.yaml` → `cmd/next*.go` view → generated `wave-orchestration` skill → host fan-out → wave-orchestration verification references → `internal/state/wave_execution.go` (`waveRunDispatchMode`, `BuildWaveRuns`) → `internal/engine/progression/wave_sync.go` (`evaluateGovernedWaveExecution`) → `ExecutionSummary.OpenBlockers` → `validate/next/status --json`.
- Blast radius: model + state + engine/wave + engine/progression + cmd + tmpl + toolgen + reason_code, plus tests. All new gates fold into the single existing assembly point `wave_sync.go:~173-180` (`incompleteBlockers`) under the `len(planDriftBlockers)==0` suppression guard; no new persistence or JSON pipeline.
- Constraints/invariants preserved: "engine governs, host executes" (no in-engine spawn); `forced`(default)/`off` parallelization model; `wave-plan.yaml` freshness hashes (`tasks_plan_hash` family) must not absorb the new view-only `Advisories`; same-wave tasks are guaranteed file-disjoint + dependency-free by `validateWaveStaticConflicts`.

### Patterns
- Reuse (do NOT reinvent): conflict predicates `normalizeTargetFileForConflict` / `targetFileContains` / `targetPatternMatches` / `targetHasPatternMeta` / `targetPatternStaticPrefix` in `internal/engine/wave/wave.go` (all unexported, no external callers → safe to wrap/export). C1 `TargetCoversPath` must wrap these so "coverage" == planner conflict semantics.
- Token-parsing idiom: `collectWaveDispatchMode` (`internal/model/wave_execution.go:294-321`) — tolerant tokenizer over verification references; C3 `ExecutorAgentHandlesFromVerification` mirrors it. Enum-with-`IsValid()` (`:271-278`) is constant-driven, so C0's value change propagates automatically.
- Blocker codes live in `canonicalReasonDefinitions` (`internal/model/reason_code.go`, e.g. `incomplete_execution_task` :209) as `{Severity, Message}`; new codes follow that shape + a comment block.
- Path normalization: `model.NormalizePublicPath` (`internal/model/wave_execution.go:186-199`) for slash/relative canonicalization on both sides of C1 comparisons.
- View duality: `cmd/next_wave_plan.go` has derived (`derivedWavePlanView`) and authoritative (`wavePlanViewFromModel`, re-exports `Parallel`) paths — C5 `Advisories` must populate both.

### Risks
- **C2 breaking (medium)**: removing the silent-parallel inference (`internal/state/wave_execution.go:505-515`) blocks in-flight started parallel waves that lack a `dispatch_mode` token until the host records it. Mitigated by C0 (literal now matches the skill the host already follows) + C4 (docs). Reversible (revert the diff).
- **C1 post-hoc (medium, inherent)**: a parallel wave's overlap/scope-escape is only detected when its evidence is evaluated — by then it may already be committed into the shared worktree (change-level isolation, per user design). Recovery = fix `target_files` + re-record evidence, or rescope. Strictly better than today's zero audit.
- **C3 cannot prove real parallelism (low, inherent)**: a coordinator can write N fake handles then run inline-serial. C3 only raises the bar (requires N distinct handles). Honest ceiling of "engine doesn't spawn".
- **Guardrail domain**: `external_api_contracts` — `WaveRun.dispatch_mode` output value changes `parallel`→`parallel_subagents`, and the validate/next/status blocker-code set grows. Reviewed as external contracts in S3.
- **False-positive risk (low)**: Windows/relative path aliasing in C1 — mitigated by normalizing both sides through `NormalizePublicPath`.

### Test Strategy
- Existing coverage to keep green: `internal/engine/wave/wave_test.go` (13 deterministic golden-wave cases incl. 8 conflict kinds), `cmd/next_wave_plan_test.go` (Parallel re-export, retired `wave:` rejection), `internal/toolgen` (README/adapter contract), `internal/tmpl/wave_isolation_content_test.go` (asserts absence of `"engine rejects"`/`"engine-level rejection"`), `thin_host_content_test.go`.
- Rewrite: dispatch-inference tests in `internal/state/wave_execution_test.go` (:277/307/418 rely on the inference branch C2 removes).
- New unit tests per gate: scope-escape fires on out-of-target file, fails closed when a planned task has empty `target_files` but records `changed_files`, and passes directory+glob-covered files (proving `TargetCoversPath` routing); overlap only on `Parallel==true` waves, not across sequential waves sharing a file; started parallel wave missing `dispatch_mode` blocked while `degraded_sequential` passes; `parallel_subagents` wave missing handle blocked while `degraded_sequential` requires none; `AnalyzeWaveNarrowingCauses` flags directory/glob + linear chain, passes honest single-task wave + pure file-conflict serialization; both preview(mutate=false) and mutate=true surface new blockers in `OpenBlockers`.
- Infra needs: none new — existing table-driven `VerificationRecord` / `WavePlan` / `TaskRun` builders suffice.

### Options
- **Approach A (SELECTED): engine-enforced fail-closed evidence gates (C0–C3) + non-blocking widening advisories (C5) + doc alignment (C4); engine governs evidence, host still executes.** Tradeoffs: makes the host's already-recorded evidence (`changed_files`/`target_files`, `dispatch_mode`, `executor_agent`) authoritative without an engine spawn runtime; smallest clean design that reuses planner conflict semantics; C2 is a deliberate, documented breaking change; widening stays advisory (human judgment via plan-audit).
- **Approach B (REJECTED): per-executor worktree isolation + worktree proof tokens** (gsd-core model). Tradeoffs: would make collisions structurally impossible, but the user has deliberately rejected per-task isolation (isolation stays change-level); large new runtime surface; contradicts the established architecture.
- **Approach C (REJECTED): gsd-core-style fail-soft — auto-downgrade an overlapping wave to sequential and continue.** Tradeoffs: never blocks, but normalizes silent loss of safety and violates the project's fail-closed principle for sensitive work; reintroduces the silent-inference disease C2 removes.

## Unknowns
- Resolved: "Does changing the `WaveDispatchParallel` constant value break parsing?" -> No — `IsValid()` switches on the constant, so the change propagates through `IsValid`→`collectWaveDispatchMode`→`waveRunDispatchMode`; verified `internal/model/wave_execution.go:271-321`.
- Resolved: "Any other consumer of the literal `parallel` as a dispatch value?" -> No — only the constant definition; `cmd/next.go:237` `parallel` is the unrelated wave-view bool field; only archived artifacts mention the output literal.
- Resolved: "Does a view-only `Advisories` field affect freshness?" -> No — freshness derives from `tasks.md`→`PlanWaves`, not the display struct; `Advisories` is omitempty view-only and excluded from `wave-plan.yaml`.
- Remaining: None blocking for plan-audit.

## Assumptions
- Same-wave tasks are file-disjoint and dependency-free - Evidence: `validateWaveStaticConflicts` / `nodeConflictsWithWave` in `internal/engine/wave/wave.go`; `internal/engine/wave/wave_test.go`.
- Host records exhaustive `changed_files` and accurate `target_files` - Evidence: wave-orchestration `SKILL.md.tmpl` input contract (:128-131); under-reporting yields false-negatives but is not weaker than today's zero audit.
- The `len(planDriftBlockers)==0` guard is the correct home for the new gates - Evidence: under plan drift the wave-plan/target mapping is stale, so suppressing the audit avoids false positives; matches existing `IncompleteExecutionTaskBlockers` placement (`wave_sync.go:173-180`).

## Canonical References
- `internal/model/wave_execution.go:186-321` (NormalizePublicPath, WaveDispatch* consts, IsValid, collectWaveDispatchMode, WaveDispatchModesFromVerification)
- `internal/state/wave_execution.go:496-521` (waveRunDispatchMode silent-inference branches)
- `internal/engine/wave/wave.go` (PlanWaves + conflict predicates + Node/Wave types)
- `internal/engine/progression/wave_sync.go:140-180` (evaluateGovernedWaveExecution, dispatch-mode collection, incompleteBlockers assembly + plan-drift guard)
- `internal/model/reason_code.go:40-685` (canonicalReasonDefinitions)
- `internal/model/types.go:196-205` (TaskRun.ChangedFiles/TargetFiles)
- `cmd/next.go:228-241`, `cmd/next_wave_plan.go` (wavePlanView/waveView, derived + authoritative paths)
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:125,195`, `references/executor-dispatch-reference.md:45`
- `internal/tmpl/wave_isolation_content_test.go:32-33` (forbidden marker guard)
- gsd-core (open-gsd/gsd-core) `src/phase.cts:423-478` (Kahn wave levels), `workflows/execute-phase.md:492-524` (intra-wave overlap downgrade — the fail-soft this change deliberately diverges from)
