# Research

## Research Findings

### Architecture
- Affected CLI entry points:
  - `cmd/next.go` builds full and compact next views, including readiness
    blockers, artifact projection, and next skill rendering.
  - `cmd/next_handoff.go` builds the compact default JSON handoff view used by
    `next --json` and non-diagnostic `run --json`.
  - `cmd/next_skill_view.go` resolves the visible next skill and currently owns
    the evidence auto-skip behavior that made `run` and `next` diverge.
  - `cmd/validate.go`, `cmd/status_view_build.go`, `cmd/health.go`, and
    `cmd/repair.go` render the diagnostic surfaces named in issue #13.
- Affected progression and evidence modules:
  - `internal/engine/progression/readiness.go` is the shared governance
    readiness aggregator and should become the common source for freshness
    details consumed by CLI surfaces.
  - `internal/engine/progression/authority.go` evaluates review layer blockers;
    it consumes exact `layer:<token>=pass|fail` references.
  - `internal/state/execution_summary.go` owns execution-summary parsing,
    freshness evaluation, and the current `ComputeTaskEvidenceInputHash`
    structural hash.
  - `internal/state/wave_execution.go`, `internal/state/store.go`, and
    `internal/state/local_runtime_paths.go` own runtime evidence paths under the
    git-common Slipway namespace.
- Dependency chains:
  - CLI commands -> `progression.EvaluateGovernanceReadiness` -> state,
    artifact, governance, scope contract, and review authority helpers.
  - `next`/`run` -> `assembleSkillViewWithOptions` ->
    `progression.ResolveNextSkill` plus required-skill evidence maps.
  - execution freshness -> `state.LoadRelevantExecutionSummaryContext` ->
    `state.ExecutionSummaryFreshness` -> engine context freshness evaluator.
- Blast radius: medium-high. The implementation touches public JSON command
  contracts, generated skills/templates, execution-summary schema, repair and
  health diagnostics, and regression tests across `cmd` and `internal/state`.
- Constraints:
  - Preserve lifecycle gate semantics. Better diagnostics must not auto-pass,
    skip reviews, or hide blockers.
  - Public JSON surfaces should derive from shared diagnostic structs rather
    than each command reconstructing explanations differently.
  - The user selected the breaking, thorough path: replace the old
    `evidence_input_hash` compatibility model with an explicit structural
    input contract.

### Patterns
- Existing command views use small exported-ish JSON structs in `cmd` and
  append richer optional fields for diagnostics. New diagnostic fields should
  follow that pattern with `omitempty` so focused tests can assert them.
- Readiness logic is already centralized in
  `progression.EvaluateGovernanceReadiness`; command-specific diagnostics
  should consume readiness results instead of duplicating freshness decisions.
- Existing reason-code patterns use stable code/detail pairs plus optional
  diagnostics. New stale-causality findings should keep stable reason codes and
  put expected/current/path/timestamp details in structured fields. The
  diagnostic shape should separate the first stale source from downstream stale
  evidence so operators do not repair symptoms before reconciling the source.
- State-layer repair and health surfaces already distinguish repairable from
  non-repairable findings. `repair --json` should extend that shape with
  applied vs unrepaired findings instead of overloading string slices.
- Generated skill templates live under `internal/tmpl/templates/skills/*.tmpl`;
  review layer examples must be corrected there, not only in runtime code.

### Risks
- High: execution-summary schema break. Removing `evidence_input_hash` without
  a compatibility layer will invalidate stale local summaries until they are
  regenerated. This is intentional for this change, but diagnostics and repair
  output must make the recovery path explicit.
- High: JSON contract churn. `next`, `validate`, `run`, `status`, `health`, and
  `repair` are AI/operator-facing API surfaces. Tests should cover shape and
  semantics, not only Go helper behavior.
- Medium: next-skill authority. If the actionable skill is derived in only one
  command path, the original issue can recur. The auto-skip/required-actionable
  skill logic should be shared.
- Medium: repair scope. Automatically rewriting all drift would be unsafe.
  Repair should only apply deterministic fixes and report unrepaired drift with
  next actions.
- Medium: linked worktree paths. Diagnostics must distinguish project root,
  bound worktree workspace, governed bundle, verification dir, and git-common
  runtime evidence dir to avoid writing recovery files to the wrong authority.
- Low: docs/templates drift. Generated skill examples can silently contradict
  gate tokens unless template tests check the emitted text.
- Guardrail domains: `external_api_contracts`, because command JSON and
  generated adapter behavior are externally consumed by AI/operator workflows.
- Reversibility: Code changes are reversible through git. Local runtime
  evidence schema changes are not safely reversible without explicit
  regeneration/repair guidance, so tests must verify that stale or old-shape
  evidence fails clearly.

### Test Strategy
- Existing coverage:
  - `cmd/progression_next_test.go` covers next-skill JSON and skill evidence.
  - `cmd/run_contract_test.go`, `cmd/lifecycle_commands_test.go`, and
    `cmd/error_contract_test.go` cover run and error surfaces.
  - `cmd/status_view_build_test.go`, `cmd/health_test.go`, and
    `cmd/repair_test.go` cover status, health, and repair JSON behavior.
  - `internal/state/execution_summary_test.go` covers execution summary
    freshness and stale blockers.
  - `internal/toolgen/toolgen_test.go` and template tests cover generated
    command/skill artifacts.
- Required new regression coverage:
  - S3 review state where spec review evidence passes but code-quality review
    is missing: `next --json`, `next --json --diagnostics`, `validate --json`,
    and `run --json --diagnostics` must agree on `code-quality-review` as the
    actionable next skill or expose an explicit structured reason if a display
    skill differs.
  - Execution summary with old `evidence_input_hash` only should not be treated
    as fresh under the new contract. Diagnostics should identify missing
    structural input fields and a regeneration/repair action.
  - Execution summary with explicit structural input fields should become stale
    when any expected field differs, with expected/current detail surfaced.
  - Cascaded freshness drift should identify the first stale source artifact,
    downstream stale evidence artifacts, and regeneration order. The issue #13
    incident chain to model is: implementation file list changes -> `tasks.md`
    update -> stale plan audit/wave plan -> stale execution summary.
  - Operator repair mistakes should be covered directly: wrong structural input
    values, manual `captured_at` changes, and manual hash or plan-alignment
    edits must produce diagnostics that identify the bad value and point back to
    regeneration or rescoping instead of further manual edits.
  - `run --resume` in `S3_REVIEW` should return `resume_unavailable` with
    `current_state`, allowed resumable states, and a review/validate action.
  - Linked worktree fixtures should assert authoritative runtime evidence path
    points under git-common `.git/slipway/runtime/...`.
  - `repair --json` should include separate applied and unrepaired finding
    collections.
  - `status --json` artifact DAG nodes should include blocking/non-blocking
    semantics for draft/ready states.
  - `health --json` codebase-map findings should indicate whether the warning
    blocks the active change.

## Alternatives Considered

- Minimal diagnostics only: keep `evidence_input_hash`, add explanatory prose
  and expected/current hash output. Lower schema churn, but it preserves the
  confusing operator-facing hash contract that caused issue #13.
- Explicit structural execution freshness contract: replace
  `evidence_input_hash` with explicit fields such as change id, run summary
  version, task id, and guardrail domain in execution evidence summaries, then
  render expected/current comparisons from those fields. This is a breaking
  but clearer model and matches the user's direction to avoid a compatibility
  layer.
- Repair-first workaround: leave schema mostly unchanged and make `repair`
  regenerate stale hashes more aggressively. This reduces initial code churn
  but keeps source inspection pressure when repair cannot safely infer intent.

Selected: Explicit structural execution freshness contract. It makes the CLI
and artifact schema explain the real contract directly, removes the misleading
legacy hash field instead of keeping compatibility behavior, and gives
diagnostics enough structure to identify wrong values without reading source.

## Unknowns
- Resolved: whether the hash should remain operator-facing -> no. The user
  selected the thorough breaking path, so planning should replace the old
  hash compatibility model with explicit structural fields.
- Resolved: whether review token examples can keep semantic substitutes such as
  `CORRECTNESS` and `SAFETY` -> no. Runtime gates consume exact `R0`, `IR1`,
  and domain-required layer tokens.
- Remaining: exact field names for the new execution-summary structural input
  object. Plan audit should lock names before implementation.
- Remaining: whether generated docs/adapters need refresh commands after
  template and command contract changes.


## Assumptions
- The new execution freshness schema may intentionally break active local
  execution summaries that only carry `evidence_input_hash`; diagnostics and
  repair output will guide regeneration rather than preserving compatibility.
  Evidence: user direction and `intent.md` Approved Summary.
- The actionable next skill should be computed once and reused by next/run
  surfaces. Evidence: `cmd/next_skill_view.go` currently houses auto-skip
  behavior and issue #13 reports divergence between next and run.
- Repair must stay bounded to deterministic local integrity fixes. Evidence:
  `cmd/repair.go` and `internal/state/execution_repair.go` already separate
  repairable state cleanup from non-repairable findings.


## Canonical References
- `artifacts/changes/resolve-github-issue-13-improve-governed-workflow-diagnostic/intent.md`
  for confirmed scope and breaking freshness-schema direction.
- `cmd/next.go` and `cmd/next_handoff.go` for next JSON surfaces.
- `cmd/next_skill_view.go` for current next-skill evidence auto-skip behavior.
- `cmd/run.go` for `--resume` validation and run diagnostics.
- `cmd/validate.go`, `cmd/status_view_build.go`, `cmd/health.go`, and
  `cmd/repair.go` for affected diagnostic JSON surfaces.
- `internal/engine/progression/readiness.go` for shared governance readiness
  and stale evidence blocker aggregation.
- `internal/engine/progression/authority.go` and
  `internal/engine/review/review.go` for review layer token requirements.
- `internal/state/execution_summary.go` for current execution-summary schema,
  freshness evaluation, and legacy structural hash calculation.
- `internal/state/store.go`, `internal/state/wave_execution.go`, and
  `internal/state/local_runtime_paths.go` for runtime evidence and git-common
  path authority.
- `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl` and
  `internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl` for
  generated review evidence examples.
