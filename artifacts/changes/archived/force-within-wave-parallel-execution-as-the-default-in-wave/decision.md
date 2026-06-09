# Decision

## Alternatives Considered
Three approaches were evaluated in `research.md`:
- **Option 1 — derived persisted signal (SELECTED):** persist a derived per-wave `parallel` flag (set at materialize from task count, excluded from freshness-hash inputs); flip the wave-orchestration skill to parallel-by-default; add `WaveRun.dispatch_mode`; add a `parallelization` config setting defaulting to forced. Tradeoff: puts the explicit signal in both `wave-plan.yaml` and `next --json` with no migration and no hash entanglement, at the cost of one derived field that must be deliberately kept out of the hash inputs.
- **Option 2 — view-only signal:** compute `parallel` only in `next --json`, never persisted. Tradeoff: zero hash risk but contradicts the requirement that the signal live in `wave-plan.yaml` and leaves no durable audit record.
- **Option 3 — first-class persisted field with version bump + migration:** Tradeoff: most explicit and schema-validated, but adds a `WavePlanVersion` migration and the highest freshness-hash entanglement risk for marginal value.

## Selected Approach
Option 1. It satisfies the Approved Summary (explicit signal in `wave-plan.yaml` and `next --json`, dispatch-mode evidence, and a `parallelization` off-switch) with the smallest clean, host-driven design. The `parallel` flag is computed in `MaterializeWavePlan` from the wave's task count (safe because `validateWaveStaticConflicts` already guarantees same-wave tasks are file-disjoint and dependency-free) and is deliberately excluded from the tasks-plan hash inputs so freshness is unaffected. The generated skill is rewritten so concurrent dispatch of a multi-task wave is the default; a host that genuinely cannot run concurrent executors records `dispatch_mode=degraded_sequential`. The engine signals and records but never executes work itself — Slipway stays host-driven, not a harness.

## Interfaces and Data Flow
- `model.WavePlanWave` gains `Parallel bool` (`yaml:"parallel,omitempty"`), set in `MaterializeWavePlanAt` and surfaced by `cmd/next_wave_plan.go` into `input_context.wave_plan.waves[]`.
- `model.WaveRun` gains `DispatchMode` (enum `parallel` | `degraded_sequential`) with `IsValid()` mirroring `WaveVerdict`; `Validate` rejects other non-empty values.
- `model.Config` gains a `parallelization` setting (default forced, accepts `off`); read where the wave-orchestration skill view and `next` wave-plan view are built so the effective mode is reflected.
- Data flow unchanged otherwise: `tasks.md` → `wave.PlanWaves` → `MaterializeWavePlan` (now stamps `Parallel`) → `wave-plan.yaml` → `next --json` → generated `wave-orchestration` skill → host fan-out → `WaveRun` evidence (now records `dispatch_mode`).
- Freshness hashes (`tasks_plan_hash`, structural/scope/semantic) keep deriving from `tasks.md` only; `Parallel` is excluded from those inputs.

## Rollout and Rollback
- Rollout is additive and lands across waves: model fields → config knob → materialize → next view → skill templates → regenerated adapters + docs. The kernel stays green between waves (each task builds and tests independently).
- Rollback: revert the change branch; the new fields are `omitempty` and the config defaults to forced, so removing them restores prior behavior with no migration. Verification command: `go build ./... && go vet ./... && go test ./... -count=1`.

## Risk
- **Freshness-hash entanglement (medium):** if `Parallel` leaked into the tasks-plan hash inputs it could strand materialized plans. Mitigation: derive at materialize, exclude from hash inputs, and add a test asserting `tasks_plan_hash` is unchanged by the flag (REQ-006).
- **Unintended behavior flip for existing repos (low, intended):** defaulting `parallelization` to forced changes the skill language for existing projects; the `off` knob is the escape hatch (REQ-005).
- **Host without concurrent executors (low):** must degrade to sequential and record `degraded_sequential`; fail-open — it never blocks `done`.
- Guardrail domains: none. Reversibility: high.
