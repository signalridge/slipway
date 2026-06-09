# Intent

## Summary
Force within-wave parallel execution as the default in wave-orchestration. The engine already guarantees same-wave tasks are dependency-free and file-disjoint (PlanWaves + validateWaveStaticConflicts), so make parallel the mandatory default rather than a host-conditional option: (1) flip the wave-orchestration skill from 'parallel when supported' to parallel-by-default; (2) add an explicit parallel signal to wave-plan.yaml and surface it in slipway next --json input_context.wave_plan; (3) record dispatch mode per wave run (parallel vs degraded_sequential) so lost parallelism is visible. Single working tree relying on the file-disjoint guarantee; no engine executor — stay host-driven, not a harness.

## Complexity Assessment
complex
<!-- Rationale: touches the engine wave model + materialization, the next --json contract, the generated wave-orchestration skill (a public surface), and adds evidence (dispatch mode) — multiple aligned surfaces, but no new sensitive domain. -->

## Guardrail Domains
none detected

## In Scope
- `internal/model/wave_execution.go`: add an explicit `parallel` signal to `WavePlanWave` (true when a wave has >1 task, since the engine guarantees they are dependency-free and file-disjoint); add a `dispatch_mode` field (`parallel` | `degraded_sequential`) to `WaveRun` with validation.
- `internal/state/wave_execution.go`: set the per-wave `parallel` flag in `MaterializeWavePlanAt`.
- `internal/model/config.go` (+ load/validate + `.slipway.yaml`): add a `parallelization` setting that defaults to forced and accepts an `off` value so a project/change can opt out of forced parallel; the wave-orchestration skill and `next --json` reflect the effective mode.
- `slipway next --json` wave-plan view (next_wave_plan / next_context build): surface the per-wave `parallel` signal in `input_context.wave_plan.waves[]`.
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` and `references/executor-dispatch-reference.md`: flip dispatch language from "parallel when the tool supports it / otherwise sequentially" to **parallel-by-default** for every wave with >1 task; require recording `dispatch_mode=degraded_sequential` when a host genuinely cannot run concurrent executors.
- Regenerated host adapter skills for all tools (`slipway init --tools all --refresh` output) so the generated surfaces match the template.
- Tests: model validation, materialization parallel flag, next --json view, and the wave-orchestration skill/toolgen contract.
- Docs: `docs/workflow.md` wave-execution wording if it describes dispatch.

## Out of Scope
- Any engine-side executor or scheduler that spawns tasks itself — Slipway stays host-driven (NOT a harness).
- Per-task worktree isolation (one worktree per parallel task) — rely on the existing file-disjoint guarantee in a single working tree.
- Changing the dependency-ordering or conflict-detection algorithm itself (already correct).
- The README competitor comparison (separate, already in PR #146).

## Constraints
- Host-driven only: the engine may compute, signal, and record, but must not execute implementation work.
- Must not break wave-plan freshness: the new `parallel` field must not silently corrupt `tasks_plan_hash` / structural-scope-semantic hashes or strand materialized plans.
- Fail-open, not fail-closed: per the chosen design, missing parallelism is RECORDED (`degraded_sequential`), it does not block `done`.

## Acceptance Signals
- `slipway next --json` at `S2_EXECUTE` shows each wave with `total_tasks > 1` carrying an explicit `parallel: true` signal in `input_context.wave_plan.waves[]`.
- Materialized `wave-plan.yaml` encodes the per-wave `parallel` flag; `WaveRun` accepts and validates `dispatch_mode` of `parallel` or `degraded_sequential`.
- The generated `wave-orchestration` skill instructs parallel-by-default and the `degraded_sequential` recording rule (asserted by a toolgen/skill contract test).
- With `parallelization: off` configured, the same surfaces reflect a non-forced mode (waves carry no forced-parallel signal / the skill does not mandate concurrency); default config keeps it forced. Covered by a test.
- `go build ./...`, `go vet ./...`, and `go test ./... -count=1` are green, including new tests.

## Open Questions
None

## Deferred Ideas
- Optional future: per-task worktree isolation for hosts that need git/build-lock safety during concurrent edits (extends `ControlWorktreeIsolation` to per-task).
- Optional future: a fail-closed gate that rejects `degraded_sequential` on multi-task waves when a stricter preset wants hard-forced parallelism.

## Approved Summary
Make within-wave parallel execution the default in Slipway's wave-orchestration. The engine already proves each wave's tasks are dependency-free and file-disjoint, so the generated wave-orchestration skill instructs hosts to dispatch all tasks in a wave concurrently by default; a host that genuinely cannot run concurrent executors falls back to sequential and records `dispatch_mode=degraded_sequential`. The engine surfaces an explicit per-wave `parallel` signal in `wave-plan.yaml` and `slipway next --json`, and `WaveRun` records the actual dispatch mode. A `parallelization` config setting (default forced) lets a project/change opt out via `off`. Single working tree (file-disjoint guarantee); no engine executor — host-driven, not a harness. Out of scope: an engine scheduler, per-task worktrees, the README comparison. Primary acceptance: `slipway next --json` at S2 marks multi-task waves `parallel: true`, `WaveRun` validates `dispatch_mode`, the generated skill asserts parallel-by-default, `parallelization: off` flips the surfaces to non-forced, and `go test ./...` is green.

Confirmed by user: 2026-06-09T08:35:58Z (chose to include the `parallelization` off-switch).
