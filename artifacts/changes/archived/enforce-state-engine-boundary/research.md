# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/state/execution_summary.go` imports `internal/engine/context` for
    evidence freshness constants, return types, and structural-input comparison
    (`internal/state/execution_summary.go:15`,
    `internal/state/execution_summary.go:286`,
    `internal/state/execution_summary.go:306`,
    `internal/state/execution_summary.go:391`).
  - `internal/state/wave_execution.go` imports `internal/engine/wave` and owns
    task-plan parsing, task-plan hashes, wave planning, and wave-plan
    materialization (`internal/state/wave_execution.go:14`,
    `internal/state/wave_execution.go:169`,
    `internal/state/wave_execution.go:182`,
    `internal/state/wave_execution.go:186`,
    `internal/state/wave_execution.go:273`).
  - `internal/architecture/dependency_direction_test.go` currently forbids
    `cmd`, `internal/tmpl`, and `internal/toolgen` from authority packages, but
    does not forbid `internal/state -> internal/engine`
    (`internal/architecture/dependency_direction_test.go:17`,
    `internal/architecture/dependency_direction_test.go:22`).
- Dependency chains:
  - `internal/state -> internal/engine/context` exists only because
    `EvidenceFreshness`, `EvidenceFreshnessInput`, and
    `EvaluateEvidenceFreshness` live in an engine package even though they are a
    generic structural freshness primitive (`internal/engine/context/context.go:13`,
    `internal/engine/context/context.go:21`,
    `internal/engine/context/context.go:26`).
  - `internal/state -> internal/engine/wave` is heavier: state computes
    `tasks.md` hashes, parses checkbox task plans, calls `PlanWaves`, and writes
    `wave-plan.yaml` (`internal/engine/wave/parse.go:47`,
    `internal/engine/wave/parse.go:76`,
    `internal/engine/wave/parse.go:80`,
    `internal/engine/wave/parse.go:84`,
    `internal/engine/wave/wave.go:24`).
  - Upper layers already call state's wave materialization API from engine and
    command surfaces, so this change must keep behavior while moving ownership:
    S1->S2 progression materializes `wave-plan.yaml`
    (`internal/engine/progression/advance_governed.go:507`,
    `internal/engine/progression/advance_governed.go:512`), and repair uses
    wave-plan state helpers (`internal/state/execution_repair.go:114`,
    `internal/state/execution_repair.go:184`,
    `internal/state/execution_repair.go:253`).
- Blast radius:
  - Public lifecycle behavior can be affected anywhere that reads execution
    freshness, materializes wave plans, repairs wave evidence, or validates
    task-plan drift.
  - Serialized files must stay compatible: `execution-summary.yaml` and
    `wave-plan.yaml` are governed/runtime artifacts, not migration targets.
- Constraints:
  - `internal/state` must remain a persistence/path/runtime read-write layer,
    not an engine lifecycle/wave planning layer.
  - Existing public function names may be kept as compatibility wrappers only
    if the production import direction is corrected; future work can simplify
    the API surface after the boundary is enforced.

### Patterns
- Existing conventions:
  - State package functions own path resolution, strict YAML/JSON loading, and
    atomic writes (`internal/state/execution_summary.go:82`,
    `internal/state/execution_summary.go:131`,
    `internal/state/wave_execution.go:30`,
    `internal/state/wave_execution.go:51`,
    `internal/state/wave_execution.go:80`).
  - Engine progression owns lifecycle decisions and already calls state when it
    needs persisted artifacts (`internal/engine/progression/advance_governed.go:505`,
    `internal/engine/progression/advance_governed.go:528`).
  - Existing architecture tests parse Go imports directly and report file-level
    violations (`internal/architecture/dependency_direction_test.go:33`,
    `internal/architecture/dependency_direction_test.go:47`,
    `internal/architecture/dependency_direction_test.go:56`).
- Reusable abstractions:
  - The generic freshness primitive can move to a lower-level package such as
    `internal/model` or a small `internal/freshness` package without requiring
    state to understand engine context.
  - Task-plan parsing, task-plan hashing, conflict-aware wave projection, and
    target coverage are currently implemented in `internal/engine/wave`, but
    they are engine-neutral planning primitives: the package depends only on
    `internal/model`, not on progression or lifecycle state. Move those
    primitives to a lower-level package and update all production consumers.
    `internal/state` may use the lower-level primitive to derive persisted
    cache metadata, but it must not import `internal/engine` or own engine
    lifecycle decisions.
- Convention deviations:
  - Some compatibility wrapper cleanup may require touching command surfaces and
    engine tests together, but the change should avoid broad lifecycle rewrites.
  - `internal/state/execution_repair.go` currently mixes repair persistence with
    wave-plan derivation. Moving wave derivation out may require relocating some
    repair orchestration to an engine-owned package or splitting pure state I/O
    from derivation logic.

### Risks
- Technical risks:
  - High: moving wave materialization can break S1->S2 transition because
    `advance_governed.go` creates `wave-plan.yaml` transactionally before
    entering S2 (`internal/engine/progression/advance_governed.go:507`,
    `internal/engine/progression/advance_governed.go:512`).
  - Medium: execution-summary freshness strings are public JSON/prose signals;
    refactoring constants must preserve exact values `fresh`, `stale`, and
    `unknown` (`internal/engine/context/context.go:15`).
  - Medium: repair and health code depend on task-plan structural/scope hashes
    to decide whether evidence is stale or repairable
    (`internal/state/execution_repair.go:184`,
    `internal/state/execution_repair.go:219`).
  - Low: architecture-test expansion is straightforward and should fail only
    on production imports because the existing test skips `_test.go`
    (`internal/architecture/dependency_direction_test.go:43`).
- Guardrail domains:
  - No auth, credentials, schema migration, or external API contract changes.
  - The main guardrail is lifecycle correctness: do not regress governed
    readiness, evidence freshness, repair, or wave execution.
- Reversibility:
  - The change is code-only and reversible by reverting the PR; persisted
    artifact formats remain unchanged.

### Test Strategy
- Existing coverage:
  - `internal/state` has focused wave-plan, execution-summary, repair, and
    health tests.
  - `internal/engine/progression` has lifecycle transition and wave execution
    tests that exercise S1->S2 materialization and freshness behavior.
  - `cmd` tests cover public status/next/evidence/repair behavior around wave
    plans and execution freshness.
- Infrastructure needs:
  - No new external services or fixtures are required.
  - Add/extend architecture tests using the existing import-parser helper.
- Verification approach:
  - First prove the current regression by confirming production
    `internal/state` imports of `internal/engine`.
  - Add a failing architecture assertion for `internal/state -> internal/engine`.
  - Refactor packages until `rg -n 'github.com/signalridge/slipway/internal/engine|internal/engine/' internal/state`
    reports no production imports.
  - Run targeted packages:
    `go test ./internal/architecture ./internal/state ./internal/engine/context ./internal/engine/progression ./cmd -count=1`.
  - Run final `go test ./... -count=1`.

### Options
- Option A: Copy freshness and wave logic into `internal/state`.
  - Tradeoff: fast local edit, but it violates the intent by making state
    understand more engine semantics and risks duplicated wave behavior.
- Option B: Move generic freshness primitives and wave/task-plan primitives to
  lower-level engine-neutral packages, keeping engine lifecycle decisions above
  state and keeping state free of `internal/engine` imports.
  - Tradeoff: touches all `internal/engine/wave` consumers, but it matches the
    package boundary, avoids duplicated scheduling logic, and keeps exact
    serialized artifact behavior.
- Option C: Keep public state wrappers while internally delegating through
  dependency injection.
  - Tradeoff: can reduce immediate churn, but import direction is still easy to
    regress and the abstractions would be heavier than the current need.
- Selected: Option B. It is the only option that directly satisfies `opt.md`
  3.1 without preserving a hidden engine dependency inside `internal/state`.
  This is not a decision to make state own lifecycle progression: state remains
  the artifact I/O owner, while the lower wave/task-plan package owns the
  engine-neutral parse/hash/projection primitives that both state and engine
  consumers need. The user's standing instruction is to make the best decision
  automatically when choices appear, so this research records Option B as
  selected.

## Unknowns
- Resolved: Is the existing codebase map relevant to this change? -> No. The
  populated `artifacts/codebase/ARCHITECTURE.md` and `STRUCTURE.md` describe
  the prior release/supply-chain change, so they are not planning authority for
  this architecture-boundary change.
- Resolved: Are there production `internal/state -> internal/engine` imports?
  -> Yes, in `execution_summary.go` and `wave_execution.go`.
- Resolved: Does the current architecture gate catch the regression? -> No, the
  forbidden import list omits `internal/engine`.
- Remaining: None.

## Assumptions
- Existing serialized artifact schemas for `execution-summary.yaml` and
  `wave-plan.yaml` must not change. Evidence: state load/save code validates
  the current model types and writes those files directly
  (`internal/state/execution_summary.go:131`,
  `internal/state/wave_execution.go:51`,
  `internal/state/wave_execution.go:80`).
- Tests may import across layers for black-box or fixture setup while the
  production architecture gate remains strict. Evidence: the current gate
  intentionally skips `_test.go` files
  (`internal/architecture/dependency_direction_test.go:43`).

## Canonical References
- `artifacts/changes/enforce-state-engine-boundary/intent.md`
- `internal/state/execution_summary.go:15`
- `internal/state/execution_summary.go:306`
- `internal/state/execution_summary.go:391`
- `internal/state/wave_execution.go:14`
- `internal/state/wave_execution.go:169`
- `internal/state/wave_execution.go:273`
- `internal/engine/context/context.go:13`
- `internal/engine/context/context.go:26`
- `internal/engine/wave/parse.go:47`
- `internal/engine/wave/wave.go:24`
- `internal/architecture/dependency_direction_test.go:22`
- `internal/engine/progression/advance_governed.go:507`
- `internal/state/execution_repair.go:114`
