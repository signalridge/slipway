# Research

## Alternatives Considered

### Architecture
- Affected modules / files:
  - `internal/model/wave_execution.go` — `WavePlan`, `WavePlanWave`, `WavePlanTask`, `WaveRun` (Normalize/Validate live here).
  - `internal/state/wave_execution.go` — `MaterializeWavePlanAt`, `LoadWavePlanForChange`, `SaveWavePlan`.
  - `internal/engine/wave/wave.go` — `PlanWaves` (dependency layering) + `validateWaveStaticConflicts` (same-wave `target_files` disjointness); `parse.go`.
  - `cmd/next_wave_plan.go` — builds `input_context.wave_plan` for `slipway next --json`.
  - `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` + `references/executor-dispatch-reference.md` — the generated public skill surface.
  - `internal/model/config.go` (+ load/validate) — `.slipway.yaml` schema, for the `parallelization` knob.
  - `internal/toolgen/` — regenerates host adapter skills from the templates.
- Dependency chain: `tasks.md` → parsed nodes → `wave.PlanWaves` → `MaterializeWavePlan` → `wave-plan.yaml` → `cmd/next_wave_plan.go` → `next --json input_context.wave_plan` → host reads `wave-orchestration` skill → `WaveRun` evidence.
- Blast radius: additive engine field + additive `next --json` field + skill wording + additive config field. Confined to S2 execution governance; no sensitive domain.
- Invariants to preserve: (1) same-wave tasks are dependency-free AND file-disjoint (the safety basis for concurrency — enforced by `PlanWaves` and hardened for path aliases, parent/child target overlaps, and case-only aliases); (2) wave-plan freshness hashes (`tasks_plan_hash`, structural/scope/semantic) are derived from `tasks.md`, NOT from the wave grouping — the new signal must not feed them; (3) engine never executes work (host-driven).

### Patterns
- Additive struct fields with `omitempty` + defaulting in `Normalize()` is the established model pattern (see `WavePlan.Normalize`).
- Enum-with-`IsValid()` is the established pattern for constrained string fields (`WaveVerdict`); `dispatch_mode` should mirror it (`parallel` | `degraded_sequential`).
- Config knobs follow `internal/model/config.go` struct + load/validate; presets live in `internal/engine/governance/preset_policy.go`.
- `ControlWorktreeIsolation` already exists but is intentionally NOT used here (single-tree decision).

### Risks
- **Wave-plan freshness entanglement (medium)** — if `parallel` were folded into the tasks-plan hash inputs it could strand materialized plans or churn freshness. Mitigation: derive `parallel` at materialize from task count and keep it OUT of the hash inputs (hashes already derive from `tasks.md`, not the wave grouping).
- **Behavior change for existing projects (low/intended)** — defaulting `parallelization` to forced flips existing repos to parallel-by-default skill language. This is the requested behavior; the `off` knob is the escape hatch.
- **Host without concurrent subagents (low)** — must degrade to sequential and record `dispatch_mode=degraded_sequential`; fail-open, never blocks `done`. Malformed, conflicting, stale, or unknown-wave advisory dispatch references are ignored instead of surfaced as naked sync errors.
- **Static target aliasing under default parallelism (medium)** — aliasing such as `./a.go` vs `a.go`, `internal\pkg\file.go` vs `internal/pkg/file.go`, a directory vs a child file, or `Foo.go` vs `foo.go` could let same-wave tasks write the same target concurrently. Mitigation: slash-normalize public artifact paths and compare same-wave targets conservatively in `validateWaveStaticConflicts`.
- Guardrail domains: none. Reversibility: high (additive + wording; revert = remove field/wording/config).

### Test Strategy
- Existing coverage: `internal/engine/wave/wave_test.go` (PlanWaves), `internal/model` wave tests, `cmd` next-wave-plan tests, `internal/toolgen/toolgen_test.go` (skill contracts), config tests.
- Infra needs: none new — extend existing suites.
- Verification approach: unit test that `MaterializeWavePlan` sets `parallel` true for >1-task waves and false otherwise; model test that `WaveRun.dispatch_mode` validates `parallel`/`degraded_sequential` and rejects junk; parser/state/sync tests that malformed, stale, or not-yet-started dispatch references fail open; `PlanWaves` tests that slash/backslash alias, parent/child, and case-only target conflicts are rejected; evidence and scope-contract tests that public paths normalize to slash form; `next --json` view test asserts the per-wave signal; toolgen/skill contract test asserts parallel-by-default + degraded_sequential wording; config test that `parallelization: off` flips the effective mode; freshness test that the new field does not change `tasks_plan_hash`.

### Options
- **Option 1 (RECOMMENDED): persist a derived `parallel` flag per wave, excluded from freshness hashes; flip skill to parallel-by-default; add `WaveRun.dispatch_mode`; add `parallelization` config knob (default forced).** Tradeoffs: matches the intent (signal in BOTH `wave-plan.yaml` and `next --json`), smallest clean design, host-driven, no migration risk if the field is kept out of the hash inputs.
- **Option 2: view-only signal (compute `parallel` only in `next --json`, not persisted in `wave-plan.yaml`).** Tradeoffs: zero hash risk, but contradicts the intent's "explicit signal in wave-plan.yaml" and gives evidence/audit nothing durable.
- **Option 3: first-class persisted field with `WavePlanVersion` bump + schema validation + migration.** Tradeoffs: most explicit and validated, but adds a version migration and the highest freshness-hash entanglement risk for little extra value.
- **Selected: Option 1** — it satisfies the Approved Summary (explicit signal in wave-plan.yaml + next --json, dispatch-mode evidence, config off-switch) with the least risk, by deriving the flag at materialize and keeping it out of the freshness-hash inputs.

## Unknowns
- Resolved: "Does adding `parallel` to the wave change `tasks_plan_hash`/structural hashes?" -> Resolve by deriving it at materialize and excluding it from hash inputs (hashes derive from `tasks.md`); confirm against the hash computation during plan-audit.
- Resolved: "Where does the off-switch live?" -> `internal/model/config.go` `.slipway.yaml` setting (default forced), reflected by the skill view + `next --json`.
- Remaining: None.

## Assumptions
- Same-wave tasks are safe to run concurrently after planning accepts them. Evidence: `internal/engine/wave/wave.go` `validateWaveStaticConflicts` rejects same-wave `target_files` overlap including slash/backslash aliases, parent/child targets, and case-only aliases; `PlanWaves` puts dependents in later waves.
- The host (Claude Code etc.) realizes parallelism via subagent fan-out; the engine only signals/records. Evidence: `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md:35,42` ("one fresh executor per task", "Spawn one `Task` subagent per task").
- Freshness hashes derive from `tasks.md`, not the wave grouping. Evidence: `internal/model/wave_execution.go:15-19` hash fields are tasks-plan-derived; to confirm exact inputs in plan-audit.

## Canonical References
- `internal/model/wave_execution.go`
- `internal/state/wave_execution.go`
- `internal/engine/wave/wave.go`
- `cmd/next_wave_plan.go`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md`
- `internal/model/config.go`
