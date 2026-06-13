# Intent

## Summary
Wave plans almost always collapse into single-task sequential chains, so parallel:true waves rarely exist to dispatch: the planning surfaces (slipway instructions tasks guidance and the plan-audit skill) carry only format requirements and an upper wave-size bound (<=5 tasks), with no wave-width construction rules. Borrow GSD planner-side mechanisms into Slipway planning surfaces: minimal-wave assignment (wave = max(dependency waves)+1; independent file-disjoint tasks share a wave), a sequential-by-default plan warning at plan-audit, shared-files-to-same-task absorption guidance, and precise target_files scoping guidance, so multi-task parallel waves become the planned default where dependencies allow.
## Complexity Assessment
complex
Rationale: retires the hand-declared `wave:` contract in tasks.md and moves wave
assignment into the engine (parser, PlanWaves, materialization, plan-audit and
instructions surfaces, docs, and test fixtures all change together), with a
self-host migration consideration for in-flight bundles.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Engine-computed wave assignment: derive each task's wave from `depends_on`
  (minimal feasible level, GSD assign_waves style: no deps -> wave 1, else
  max(dependency waves)+1) and bump later for `target_files` conflicts with
  earlier-assigned tasks, reusing the existing conflict semantics in
  `internal/engine/wave/wave.go` (exact, parent/child, case-insensitive, glob).
  Assignment is deterministic with task-ID-ordered tiebreaks.
- Retire the hand-declared `wave:` task metadata: the tasks.md parser rejects a
  `wave:` declaration fail-closed with a public error that names the retirement
  and the remediation (declare real `depends_on`; the engine now assigns waves).
- Planning-surface rewrite with new-drift-vector guidance: `slipway
  instructions tasks` guidance and the plan-audit skill drop the wave authoring
  requirement and instead teach honest dependencies (`depends_on` is a
  scheduling input, not narrative order), precise `target_files` (exact files
  over directories/globs, since broad scopes flatten waves via conflict
  bumping), and the GSD shared-files-to-same-task absorption rule; the
  plan-audit dependency-integrity dimension upgrades from reference validity to
  dependency-necessity judgment.
- Aligned surfaces and tests as one product: instructions exemplars, plan-audit
  and (if it references wave authoring) wave-orchestration templates,
  docs/workflow.md, engine/parser/cmd/toolgen tests and fixtures.
- Self-host migration: this change's own tasks.md is authored under the old
  contract and re-authored (wave declarations dropped) once the new engine
  behavior lands in this worktree, following the #114 mid-flight re-walk
  precedent.

User decisions at intake:
- Round 1: wave construction is owned by the engine ("引擎全接管 wave"), not by
  prose guidance, an advisory, or a hard gate on hand-written wave numbers.
- Round 2: the `wave:` field is removed outright — fail-closed rejection, no
  ignore-one-version compatibility window, no manual override. Intentional
  serialization is expressed only through honest `depends_on` declarations.
- Round 3: the surface rewrite includes the guidance refresh for the new drift
  vectors (over-declared depends_on, over-broad target_files), with no new
  engine fully-sequential advisory signal.

## Out of Scope
- Dispatch/runtime side: the wave-orchestration executor dispatch contract,
  dispatch evidence vocabulary (`dispatch_mode:*`, `executor_agent:*`,
  `degraded_sequential`), and per-host runtime behavior stay unchanged.
- `execution.parallelization` config semantics and the per-wave `parallel` flag
  formula (`forcedParallel && len(tasks) > 1`) stay unchanged.
- No GSD-style per-executor worktree isolation and no plan-level execution
  granularity; the #184 single-worktree decision stands.
- No new engine advisory for fully-sequential computed plans (host judgment at
  plan-audit covers it).
- The plan-audit soft warning for oversized waves (>5 tasks) keeps its current
  policy; this change does not re-tune wave-size limits.

## Constraints
- Computed assignment must be deterministic (stable ordering by task ID) so
  wave-plan freshness hashes stay reproducible.
- Existing engine safety invariants stay intact: same-wave tasks remain
  dependency-free and file-disjoint by construction; cycle detection remains a
  hard error surfaced through the existing wave-plan parse_error path.
- The change that retires the wave declaration is itself governed by a tasks.md
  authored under the current contract; the migration path for in-flight bundles
  (including this change's own) must be resolved before implementation.

## Acceptance Signals
- Engine: a tasks.md with three independent file-disjoint tasks plus one task
  depending on all three materializes (without any wave declarations) as
  wave-plan.yaml wave 1 x3 `parallel: true` + wave 2 x1 `parallel: false`;
  a pure `depends_on` chain still yields N single-task sequential waves;
  target-file overlap (exact/parent-child/case/glob) bumps the later task to a
  later wave deterministically. Covered by unit tests in the wave package plus
  a CLI-level materialization test.
- Contract: a tasks.md carrying a `wave:` declaration fails parsing with the
  retirement error and actionable remediation text (no silent ignore).
- Surfaces: `slipway instructions tasks` output and the regenerated plan-audit
  skill no longer require wave authoring and carry the honest-deps /
  precise-targets / absorption guidance; toolgen and template contract tests
  pass.
- Repository health: gofmt clean, `go build ./...`, `go vet ./...`,
  `go test ./...` all green in this worktree.
- Dogfood: this change's own bundle is re-walked under the new contract and its
  final wave plan contains at least one multi-task `parallel: true` wave if its
  re-authored tasks permit one honestly.

## Open Questions
- [x] How exactly does the current tasks.md parser ingest the `wave:` metadata
  (dedicated field vs generic key), and what is the cleanest fail-closed
  rejection point with a public error + remediation? → Resolved in research.md:
  whitelisted key parsed to WaveIndex; retirement = remove from
  `allowedMetadataKeys` + dedicated retirement error in `applyStrictMetadata`.
- [x] Full inventory of surfaces that reference wave authoring (instructions
  guidance/exemplars, plan-audit, wave-orchestration templates, tdd-governance,
  docs/workflow.md, README contract tokens, test fixtures) to keep the product
  surface aligned in one pass. → Resolved in research.md (Architecture +
  Unknowns): engine trio (parse.go/wave.go/validation.go:96) + instructions:87 +
  plan-audit:90,133 + docs/workflow.md + test fixtures; other "wave" mentions
  are execution-context phrasing, not authoring contract.
- [x] Deterministic conflict-bump algorithm details: iteration order, fixpoint
  behavior when bumping creates new same-wave neighbors, and interaction with
  the wave-plan freshness hashes. → Resolved in research.md: task-ID-ordered
  topological assignment, max(dep)+1 with monotone forward bump (terminates,
  deterministic); all three hash modes currently embed declared wave
  (parse.go:109-112) so the hash function changes once — one-time staleness
  handled by existing public recovery flows.
- [x] In-flight and archived bundle exposure. → Resolved in research.md: nine
  active-change parse sites hit the retirement error until legacy `wave:` lines
  are removed; archived bundles are not re-parsed; this change's own bundle is
  the migration dogfood (#114 precedent).

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
Replace hand-declared wave numbers with engine-computed wave assignment: the
engine derives each task's wave from `depends_on` (minimal feasible level: no
dependencies -> wave 1, otherwise max(dependency waves)+1) and bumps tasks to
later waves on `target_files` conflicts using the existing conflict semantics
(exact, parent/child, case-insensitive, glob), deterministically with
task-ID-ordered tiebreaks. The `wave:` task metadata is retired outright: the
tasks.md parser rejects it fail-closed with a public error and remediation
(declare real `depends_on`; the engine assigns waves), with no compatibility
window and no manual override — intentional serialization is expressed only
through honest `depends_on`. The planning surfaces (`slipway instructions
tasks`, plan-audit, and any other wave-authoring references) are rewritten in
the same change to teach the new drift vectors: honest dependencies, precise
`target_files` (exact files over directories/globs), and the
shared-files-to-same-task absorption rule, upgrading plan-audit's
dependency-integrity check to a dependency-necessity judgment.

Out of scope: the executor dispatch contract and dispatch evidence vocabulary,
`execution.parallelization` semantics and the `parallel` flag formula,
per-executor worktree isolation, a new engine fully-sequential advisory, and
the >5-task wave-size warning policy.

Primary acceptance signal: a tasks.md with no wave declarations and three
independent file-disjoint tasks plus one dependent task materializes as
wave 1 x3 `parallel: true` + wave 2 x1; a tasks.md carrying `wave:` fails
parsing with the retirement error; full build/vet/test green; this change's own
bundle is re-walked under the new contract (self-host migration, #114
precedent).

User confirmed: 2026-06-12T16:42:21Z (intake rounds 1-3 recorded under In
Scope; summary confirmed via explicit selection "确认").
