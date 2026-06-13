# Decision

## Alternatives Considered
- **Option A — compute at the parse boundary**: `ParseTaskPlan` assigns waves
  immediately after parsing, so every consumer sees computed waves. Rejected:
  the format layer absorbs scheduling semantics, dependency-cycle detection
  becomes a "parse error" (wrong category for a graph property), and all nine
  parse call sites (tasks_contract, validation, scopecontract, status,
  governance health, wave_sync, state, next_wave_plan) pay graph-computation
  cost most of them never need.
- **Option B — compute inside `PlanWaves` (parse stays format-only)**: the
  parser drops the `wave` key and gains a dedicated retirement error;
  `PlanWaves(nodes)` derives the assignment; preview and materialization both
  flow through the one authority. Smallest clean diff (parse.go + wave.go +
  one validation blocker + surface texts).
- **Option C — compute only at materialization (S1→S2)**: `PlanWaves` keeps
  validating declared waves and a new compute path runs only inside
  `MaterializeWavePlan`. Rejected: the pre-S2 preview in `cmd/next_wave_plan.go`
  would either duplicate the algorithm or show no waves — two-implementation
  drift for no benefit.
- Prose-only guidance, an engine advisory on hand-declared waves, and a hard
  gate on non-minimal declared waves were considered at intake and rejected by
  the user in favor of full engine ownership (intent.md, intake rounds 1-2).

## Selected Approach
Option B, user-confirmed at the research stage. `ParseTaskPlan` stays a pure
format parser: `wave` leaves `allowedMetadataKeys` and gains a dedicated
retirement case in `applyStrictMetadata` whose error names the task, the
retirement, and the remediation (delete the line; declare real `depends_on`).
`PlanWaves` becomes constructive: it orders tasks topologically with task-ID
tiebreaks (reusing `compareNodesByTaskID`), assigns
`wave = max(dependency waves) + 1` (1 for roots), then bumps a task forward
while its wave contains an already-assigned task whose `target_files` conflict
under the existing `targetFilesConflict` semantics. Cycle and
unknown-dependency detection stay in `PlanWaves` as hard errors surfaced
through the existing wave-plan parse_error/blocker path.
`validateWaveStaticConflicts` is retained as an internal invariant check on
the computed output. The `plan_dimension_execution_missing_wave` blocker, its
reason-code registry entry, and its remediation vocabulary are removed. The
task-plan hash entries drop the wave field in all three modes. Planning
surfaces (`cmd/instructions.go` tasks guidance, plan-audit skill template,
docs/workflow.md) are rewritten to the computed-wave contract with the
honest-dependency, precise-target-files, and same-file-absorption guidance.

## Interfaces and Data Flow
- tasks.md (authored: objective, `depends_on`, `target_files`, `task_kind`,
  `covers`, no `wave`) → `ParseTaskPlan` → nodes → `PlanWaves` (compute +
  invariant check) → `MaterializeWavePlanAt` → wave-plan.yaml (`wave_index`,
  `parallel` via unchanged `ApplyEffectiveParallel`) → `slipway next --json`
  `input_context.wave_plan` → wave-orchestration dispatch. The preview path
  (`cmd/next_wave_plan.go`) consumes the same `PlanWaves` output pre-S2.
- Public JSON shapes do not change: `waves[].wave_index`, `waves[].parallel`,
  and task views keep their fields; only the provenance of `wave_index`
  changes from declared to computed.
- Error surfaces: parser retirement error (with remediation) for legacy
  `wave:` lines; `PlanWaves` hard errors for cycles/unknown deps; no new
  reason codes introduced; one reason code retired.

## Rollout and Rollback
- Rollout is a single atomic change: engine + validation vocabulary + surface
  texts + docs + tests land together so no surface teaches the retired
  contract.
- One-time freshness impact: hash functions stop embedding the wave value, so
  previously stored TasksPlan*Hash values mismatch recomputation once. Active
  bundles recover through the existing public staleness flow
  (re-materialization on S1→S2, `slipway repair`, or `pivot --rescope`);
  archived bundles are not re-parsed and stay frozen.
- Self-host migration (dogfood): this change's own tasks.md is authored under
  the old contract (declared waves chosen to match the computed assignment).
  After implementation lands, the worktree binary becomes lifecycle authority,
  its parser rejects our own `wave:` lines, and we follow the public
  remediation: delete the lines and re-walk the reopened stages (#114
  precedent). Any friction found in that walk is product feedback for this
  change, not a workaround to normalize.
- Rollback: pure code+text revert restores the declared-wave contract;
  wave-plan.yaml re-materializes from tasks.md on the next S1→S2 pass in
  either direction; no persisted-data migration exists.

## Risk
- Hash semantics change strands in-flight bundles once (HIGH impact, LOW
  frequency): mitigated by the documented public recovery paths and proven by
  this change's own re-walk; explicitly no special-case shim (REQ-005).
- Legacy `wave:` lines in other active worktrees fail closed at next touch
  (MEDIUM): the retirement error carries the exact remediation; archived
  bundles unaffected.
- Computed waves can be wider than authors previously declared (MEDIUM):
  execution-side safety is owned by the unchanged dispatch contract
  (target-overlap preflight, post-result conflict check, integration gate);
  the >5-task plan-audit soft warning stays as the width pressure valve.
- Bump-loop correctness (LOW): monotone forward bumps in a fixed processing
  order terminate (bounded by task count) and are deterministic;
  property-style tests plus the retained invariant check cover regressions.
- Surface drift (LOW): template-pinning and toolgen contract tests are updated
  in-change; README carries no authored-wave tokens (verified during
  research).
