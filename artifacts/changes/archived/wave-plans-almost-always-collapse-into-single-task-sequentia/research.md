# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/engine/wave/parse.go` — tasks.md checkbox-native parser. `wave`
    is a whitelisted metadata key (`allowedMetadataKeys`, parse.go:145-154)
    parsed into `Node.WaveIndex` via `parsePositiveInt` (parse.go:274-279);
    unknown keys already fail closed in `applyStrictMetadata` (parse.go:243-245).
    `HasDeclaredWave` (parse.go:48-50) exposes the declaration.
  - `internal/engine/wave/wave.go` — `PlanWaves` (wave.go:56-110) currently
    requires a declared wave per task ("missing required wave declaration",
    wave.go:71-73), validates deps point to earlier waves (wave.go:94-96), and
    hard-errors on same-wave target conflicts via `validateWaveStaticConflicts`
    (wave.go:112-134) with exact/parent-child/case-insensitive/glob semantics
    (wave.go:136-227).
  - `internal/engine/progression/validation.go:96-99` — emits
    `plan_dimension_execution_missing_wave:<id>` when a task lacks a declared
    wave; this blocker retires with the field.
  - `internal/state/wave_execution.go` — `MaterializeWavePlanAt` →
    `wave.PlanWaves`; `ApplyEffectiveParallel` sets
    `Parallel = forcedParallel && len(tasks) > 1` (unchanged).
  - `cmd/next_wave_plan.go` — pre-S2 preview and S2 authoritative read; the
    preview already derives `WaveIndex: i + 1` from `PlanWaves` output
    (next_wave_plan.go:82), so it inherits computed assignment automatically.
  - Planning surfaces: `cmd/instructions.go:87` (tasks authoring guidance
    currently mandates `wave`), `internal/tmpl/templates/skills/plan-audit/SKILL.md:90,133`
    (bundle audit metadata list + 8D Completeness require `wave`), 8D item 3
    wording ("references valid earlier-wave tasks"), `docs/workflow.md` wave
    wording.
- Dependency chains: parser → (`tasks_contract.go`, `validation.go`,
  `scopecontract/evaluate.go`, `status/view.go`, `governance/health.go`,
  `wave_sync.go`, `state/wave_execution.go`, `cmd/next_wave_plan.go`) →
  wave-plan consumers. Verified: every consumer outside the wave package reads
  the computed plan (`WavePlanWave.WaveIndex`, wave runs), never the declared
  per-task wave; the only `HasDeclaredWave` consumer is validation.go:96.
- Blast radius: wave package (parse + plan), one validation blocker, planning
  surface texts, docs, and test fixtures. No public JSON shape changes:
  `next --json` wave views are already computed output.
- Constraints: same-wave tasks remain dependency-free and file-disjoint — now
  by construction instead of by validation; `parallel` flag formula and
  `execution.parallelization` semantics unchanged; computed assignment must be
  deterministic so repeated materialization of the same tasks.md is stable.

### Patterns
- Existing conventions to reuse:
  - Conflict semantics: `targetFilesConflict` + `normalizeTargetFileForConflict`
    (wave.go:136-227) are reused verbatim as the bump predicate.
  - Deterministic ordering: `compareNodesByTaskID` (wave.go:13) already
    establishes task-ID ordering as the tiebreak convention.
  - Fail-closed parser: `applyStrictMetadata`'s unknown-key rejection is the
    established pattern; the `wave` retirement adds a dedicated case with a
    remediation-bearing message instead of falling back to the generic
    unknown-key error.
  - Blocker vocabulary: `plan_dimension_*` reason codes in validation.go; the
    retirement removes `plan_dimension_execution_missing_wave` rather than
    repurposing it.
- Borrowed mechanism (GSD): `gsd-planner.md:937-961` assign_waves — no deps →
  wave 1, else max(dependency waves)+1; files_modified overlap bumps the later
  plan; "shared files → same plan" absorption guidance. Borrow the constructive
  algorithm and the absorption guidance; do NOT borrow GSD's runtime fail-soft
  degrade (overlap → serialize wave), which cannot occur here because computed
  waves are conflict-free by construction.
- Convention deviation: none — computation lands inside the package that
  already owns grouping and validation.

### Risks
- High — freshness-hash function change: all three task-plan hash modes embed
  `"wave": task.WaveIndex` in their entries (parse.go:109-112) before the mode
  switch. Removing the field changes hash values for every tasks.md, so
  TasksPlan*Hash values stored in existing wave-plan.yaml / evidence freshness
  inputs mismatch recomputation under the new binary even when tasks.md is
  unchanged. Exposure: active in-flight bundles only; recovery is the existing
  public staleness flow (re-materialization on S1→S2, `slipway repair`, or
  `pivot --rescope` for mid-S2 plans). Archived bundles are not re-parsed
  (cmd/health.go archived scan reads lifecycle events, not tasks.md).
- Medium — legacy `wave:` lines in active bundles (including this change's own
  tasks.md) fail parsing at first touch under the new binary. Mitigation: the
  dedicated retirement error names the remediation (delete the `wave:` lines;
  declare real `depends_on`); this change re-walks its own bundle as the
  dogfood proof (#114 mid-flight re-walk precedent).
- Medium — wider waves change execution texture: computed plans can produce
  waves wider than authors used to declare; the existing plan-audit >5-task
  soft warning (unchanged policy) remains the pressure valve.
- Low — determinism/termination of the bump loop: processing in
  task-ID-ordered topological order with monotone forward bumps terminates
  (wave index strictly increases per bump, bounded by task count) and is
  deterministic; property-style unit tests cover it.
- Guardrail domains: none (no auth/credentials/financial/schema/irreversible
  surface).
- Reversibility: pure code+surface change, no persisted-data migration; a
  revert restores the declared-wave contract; wave-plan.yaml re-materializes
  from tasks.md on the next S1→S2 pass either way.
- Compile-safety note: wider waves increase concurrent executor counts, but
  the dispatch-side shared-worktree rules (no concurrent repo-wide build/test
  inside executors; post-wave integration gate) already own that risk and are
  out of scope here.

### Test Strategy
- Existing coverage: `internal/engine/wave/parse_test.go` (declared-wave
  assertions, e.g. parse_test.go:264), `wave_test.go` static-conflict and
  declared-ordering tests, `cmd/next_wave_plan_test.go` preview tests,
  `internal/engine/progression` validation tests for
  `plan_dimension_execution_missing_wave`, toolgen/template contract tests
  (`internal/tmpl/thin_host_content_test.go`, `internal/toolgen/toolgen_test.go`)
  pinning surface texts.
- Infrastructure needs: none new — table-driven unit tests in the wave package
  suffice; one CLI-level materialization test exercises the acceptance scenario
  end to end.
- Verification approach per acceptance criterion:
  - 3 independent file-disjoint tasks + 1 task depending on all three, no wave
    declarations → assert wave-plan.yaml = wave 1 ×3 `parallel: true` +
    wave 2 ×1 `parallel: false` (unit + CLI-level).
  - Pure `depends_on` chain → N single-task sequential waves.
  - Target overlap (exact, parent/child, case-only, glob) between otherwise
    independent tasks → deterministic bump of the task-ID-later task; repeated
    runs produce identical plans.
  - Dependency cycle → hard error through the existing wave-plan
    parse_error/blocker path.
  - `wave:` line present → parse fails with the retirement error and
    remediation text (no silent ignore).
  - Rewritten surfaces: instructions/plan-audit no longer require wave and
    carry honest-deps / precise-targets / absorption guidance; template and
    toolgen contract tests updated and green.
  - Repository health: gofmt clean, `go build ./...`, `go vet ./...`,
    `go test ./...` green in this worktree.

### Options
- Option A — compute at the parse boundary (ParseTaskPlan assigns waves):
  single computation site, but the format layer absorbs scheduling semantics,
  cycle detection becomes a "parse error" (semantic confusion), and all nine
  parse call sites pay graph-computation cost they do not need.
- Option B — compute inside PlanWaves (parse stays format-only): parser drops
  the `wave` key and gains the dedicated retirement error; `PlanWaves(nodes)`
  assigns waves in task-ID-ordered topological order
  (`wave = max(dependency waves)+1`, then bump while conflicting with an
  already-assigned same-wave task, reusing `targetFilesConflict`); cycle
  detection stays where dependency validation already lives; preview,
  materialization, and every consumer stay consistent through the one
  authority. Smallest clean diff: parse.go + wave.go + one validation blocker.
- Option C — compute only at materialization (S1→S2): leaves PlanWaves as a
  validator, so the pre-S2 preview either duplicates the algorithm or shows no
  waves; two-implementation drift risk for no benefit.
- Selected: **Option B** (user-confirmed at research stage). Rationale:
  PlanWaves already owns grouping and validation; computing there keeps one
  authority for preview and materialization, reuses the existing conflict and
  ordering conventions, and minimizes the diff.

## Unknowns
- Resolved: How does the parser ingest `wave:` today? → Dedicated whitelisted
  key parsed to `WaveIndex` (parse.go:145-154, 274-279); unknown keys already
  fail closed, so retirement = remove the key + add a dedicated retirement
  error case in `applyStrictMetadata`.
- Resolved: Which surfaces reference wave authoring? → Engine:
  parse.go/wave.go/validation.go:96 (blocker). Texts: cmd/instructions.go:87,
  plan-audit SKILL.md:90 and :133 (+ 8D item 3 wording), docs/workflow.md
  wording. Tests/fixtures: parse_test.go, wave tests, next_wave_plan_test.go,
  validation tests, thin_host_content_test.go, toolgen tests/goldens. Other
  template mentions of "wave" (tdd-governance, wave-orchestration dispatch,
  spec-compliance) are execution-context phrasing, not authoring contract.
- Resolved: Deterministic algorithm details and hash interaction? → Task-ID
  topological order, max(dep)+1, monotone conflict bump; terminates and is
  deterministic. All three hash modes currently embed the declared wave
  (parse.go:109-112); the field leaves the hash inputs, which changes hash
  values — handled as a one-time staleness event through existing public
  recovery flows.
- Resolved: In-flight/archived exposure? → Nine active-change parse sites
  (inventory above) hit the retirement error until legacy `wave:` lines are
  removed; archived bundles are not re-parsed. This change's own bundle is the
  migration dogfood.
- Remaining for plan-audit: exact retirement-error wording and the
  reason-code/remediation surface for legacy bundles; final wording of the
  honest-deps / precise-targets / absorption guidance blocks.

## Assumptions
- The installed CLI (0.21.0 @ 5972943) is behaviorally identical to this
  worktree's HEAD for lifecycle operations until the change lands; afterwards
  the worktree-built binary is the lifecycle authority for this bundle's
  re-walk. Evidence: `git show --stat dc49d59` (CHANGELOG-only delta).
- No consumer outside the wave package reads the declared per-task wave.
  Evidence: `rg HasDeclaredWave` → parse.go, parse_test.go, validation.go:96
  only; `rg WaveIndex` over scopecontract/status/governance/wave_sync/
  next_wave_plan shows computed-plan reads only.
- Same-wave safety invariants stay enforceable as internal invariants after
  computation (conflict-free by construction). Evidence: bump predicate is the
  same `targetFilesConflict` used by today's validator.

## Canonical References
- `internal/engine/wave/parse.go:48-50,109-112,145-154,243-245,274-279`
- `internal/engine/wave/wave.go:13,56-110,112-134,136-227`
- `internal/engine/progression/validation.go:96-99`
- `internal/state/wave_execution.go` (`MaterializeWavePlanAt`,
  `ApplyEffectiveParallel`, `EffectiveForcedParallel`)
- `cmd/next_wave_plan.go:82,115`
- `cmd/instructions.go:87`
- `internal/tmpl/templates/skills/plan-audit/SKILL.md:90,126-134`
- `docs/workflow.md:7`
- `~/ghq/github.com/open-gsd/gsd-core/agents/gsd-planner.md:937-961` (borrowed
  assign_waves mechanism; fail-soft degrade deliberately not borrowed)
- `artifacts/changes/archived/resolve-github-issue-184-add-gsd-style-automatic-subagent-di/`
  (single-worktree parallelization decision; 6×1 sequential wave plan as the
  observed collapse evidence)
