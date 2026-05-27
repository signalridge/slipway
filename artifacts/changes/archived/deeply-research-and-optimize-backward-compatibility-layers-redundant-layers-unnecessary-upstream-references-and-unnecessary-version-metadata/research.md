# Research

## Research Findings

### Architecture

- Affected modules:
  - CLI progression surfaces: `cmd/next.go:276-280`, `cmd/run.go:86-90`,
    `internal/engine/progression/advance.go:43-62`, and
    `internal/engine/progression/advance_governed.go:49-60`.
  - Task evidence and execution summaries:
    `internal/engine/progression/wave_sync.go:21-35`,
    `internal/engine/progression/wave_sync.go:266-329`,
    `internal/state/execution_summary.go:58-80`, and
    `internal/engine/progression/evidence.go:97-108`.
  - Artifact metadata and archive rewriting:
    `internal/model/types.go:128-135`,
    `internal/engine/artifact/manager.go:155-185`,
    `internal/engine/artifact/manager.go:1322-1330`, and
    `internal/state/lifecycle.go:56-152`.
  - Generated adapter cleanup:
    `internal/toolgen/toolgen.go:1057-1107` and
    `internal/toolgen/toolgen_test.go:1107-1142`.
  - Config and advisory policy parsing:
    `internal/model/config.go:15-24`,
    `internal/model/config.go:244-280`,
    `internal/model/config.go:349-356`, and
    `internal/engine/governance/policy_pack.go:18-78`.
  - Documentation and archived evidence:
    `docs/design.md:37-49`, `docs/workflow.md:99-109`, and tracked
    archived bundle files under `artifacts/changes/archived`.

- Dependency chains:
  - `next`/`run` command flags flow into `buildNextView`, then
    `advanceIfReady`, then `progression.Advance`/`AdvanceGoverned`. The
    `QuickMode` option is not merely display metadata; it disables advisory
    controls during progression.
  - Task evidence JSON is parsed by `ParseTaskEvidence`, converted into
    `ExecutionTaskSummary`, persisted into execution-summary state, and then
    consumed by readiness, review, health, and run-version-bound skill checks.
  - Artifact state is stored in `change.yaml`, updated by artifact
    reconciliation, frozen during archive, and used by readiness/status
    projections. `ArtifactState.Path` is operational; `ArtifactState.Version`
    currently has no observed consumer beyond defaulting.
  - Generated host files are projections from source registries. `toolgen` owns
    marker-gated cleanup, while generated `.codex`/`.claude`/`.opencode`
    surfaces are not independent source authorities.

- Blast radius:
  - Low-to-medium for removing `--quick`, because it is exposed on subcommands
    but intentionally hidden from root help (`cmd/root_help_test.go:18-20`).
  - Medium for tightening task evidence parsing, because external/manual task
    evidence writers may rely on defaults even though the tracked archived task
    evidence does not.
  - Medium for removing artifact `version` metadata, because archived
    `change.yaml` fixtures contain `version: 1` entries and tests may assert
    serialized artifact state.
  - Medium-to-high for changing archived path generation, because
    `rewriteArchivedArtifactPaths` currently writes archive-root joined paths
    and `internal/state/lifecycle_test.go:95` expects the absolute archived
    artifact path.
  - Low for reducing product docs' upstream comparison material and replacing
    stale workflow examples.

- Constraints:
  - Keep `change.yaml` as current-state authority and lifecycle events as
    append-only trace.
  - Do not introduce another user-facing mode as the solution.
  - Preserve generated cleanup safety: marker-gated, allowlisted, no deletion
    of unknown user-managed adapter files.
  - Keep schema/run-version metadata that gates freshness or persisted
    evolution.

### Patterns

- Intentional compatibility layers:
  - Missing lifecycle logs are treated as empty so older changes remain
    readable, and zero event version is normalized to `1`
    (`internal/state/lifecycle_event.go:121-139`).
  - Active change resolution falls back to exactly one unbound active change
    only after failing to match a bound worktree
    (`internal/state/store.go:553-588`).
  - Verification read paths prefer authoritative active/archived bundles and
    only use local fallback paths for display or archived lookup
    (`internal/state/verification.go:61-70`,
    `internal/state/verification.go:187-191`).
  - OpenCode nested command cleanup removes only known generated legacy files
    and only when the generated adapter marker exists
    (`internal/toolgen/toolgen_test.go:1107-1142`).

- Intentional projection layers:
  - Default `next` JSON is a narrow handoff view, while diagnostics retains the
    fuller internal view (`cmd/next_handoff.go:12-25`,
    `cmd/next_handoff.go:172-208`).
  - Default `status --json` removes raw governance internals and emits a compact
    governance summary (`cmd/status_json.go:27-43`,
    `cmd/status_json.go:80-99`).
  - Capability registry and surface policy are duplicated by design: registry
    owns internal skill identity and bindings, while `surfaces.go` owns public
    selectors (`internal/engine/capability/registry.go:1-17`,
    `internal/engine/capability/surfaces.go:8-24`).

- Retired compatibility guards:
  - Tests reject old route aliases or retired fields rather than preserving
    them, for example `second-opinion`
    (`cmd/route_flags_test.go:56-67`), `intent_degrade_reason`
    (`cmd/new_test.go:626-631`), and retired skill frontmatter fields
    (`internal/toolgen/toolgen_test.go:1278-1309`). These are regression
    guards, not runtime cleanup targets.

- Unnecessary or questionable layers:
  - `--quick` is a runtime bypass layer, not just a compatibility shim. It
    disables advisory controls in memory for an invocation
    (`internal/engine/progression/advance_governed.go:49-60`).
  - `TaskEvidencePayload` accepts both nested `task_run` and flat fields, then
    derives missing values from filenames, expected run versions, defaults, or
    file mtime (`internal/engine/progression/wave_sync.go:21-35`,
    `internal/engine/progression/wave_sync.go:266-329`).
  - `ManifestVersion` is set to `1` but only appears in
    `internal/engine/artifact/manager.go:155-185` and
    `internal/engine/artifact/manager.go:209-214`; no template consumer was
    found.
  - `ArtifactState.Version` is stored and defaulted
    (`internal/model/types.go:128-135`,
    `internal/engine/artifact/manager.go:1322-1330`), but no behavioral
    consumer was found.
  - `LoadAdvisoryPolicyPack` accepts multiple aliases for the same concepts and
    requires `version`/`schema_version` even though policy packs are advisory
    and bounded (`internal/engine/governance/policy_pack.go:18-78`).

### Risks

- High risk if done without migration checks:
  - Making archived artifact paths relative by changing archive persistence.
    The current path behavior is baked into archive code and tests
    (`internal/state/lifecycle.go:105-150`,
    `internal/state/lifecycle_test.go:95`). This should not be mixed into the
    first one-pass cleanup unless the implementation explicitly updates archive
    read/write semantics and tests.

- Medium risk:
  - Removing `--quick` may break private scripts that discovered the flag, but
    it is undocumented in root help and directly weakens advisory governance.
  - Tightening task evidence parsing may break untracked manually authored
    evidence. Tracked evidence is compatible with strict flat shape: `find
    artifacts/changes/archived -path '*/evidence/tasks/*/*.json' -type f | wc
    -l` returned `6`, each tracked task evidence JSON has `task_id`,
    `run_summary_version`, and `captured_at`, and `rg '"task_run"'
    artifacts/changes/archived internal` returned no tracked evidence use.
  - Removing `ArtifactState.Version` changes serialized `change.yaml` output and
    requires archived fixture updates.

- Low risk:
  - Removing unused `ManifestVersion`.
  - Replacing the stale OpenCode flat/nested example in `docs/workflow.md`.
  - Compressing or moving `docs/design.md` external comparison prose.
  - Normalizing archived research references from machine-local `ghq` paths to
    source labels or project names, as long as archive intent is preserved.

- Guardrail domains:
  - No sensitive guardrail domain is directly modified by the proposed cleanup.
    The risk is governance correctness, compatibility, and evidence freshness.

- Reversibility:
  - CLI flag removal, evidence parser strictness, artifact metadata removal, and
    docs/reference cleanup are source-controlled and reversible.
  - Archive path-generation changes are also reversible but have wider impact on
    persisted archive semantics; defer unless explicitly approved.

### Test Strategy

- Existing coverage:
  - CLI contract tests cover route/focus rejection, root help, new/status/next
    JSON contracts, and governed progression.
  - `internal/engine/progression/wave_sync_test.go` covers task evidence
    parsing, execution summary generation, stale evidence, and run-version
    mismatch.
  - `internal/toolgen/toolgen_test.go` covers generated adapter cleanup,
    generated skill frontmatter, and retired-field regression guards.
  - `internal/state/lifecycle_test.go` covers archive movement and artifact path
    rewriting.
  - `internal/model/model_test.go` covers config unknown top-level preservation.

- Required focused verification if implementation is confirmed:
  - `go test ./cmd ./internal/engine/progression -run 'Quick|Next|Run|Wave|TaskEvidence|Advance' -count=1` after removing `--quick` and tightening evidence.
  - `go test ./internal/engine/artifact ./internal/model ./internal/state -run 'Artifact|Lifecycle|Config|Policy|ExecutionSummary' -count=1` after artifact metadata/archive-facing changes.
  - `go test ./internal/toolgen -run 'OpenCodeRefresh|GeneratedAdapter|Frontmatter|Retired' -count=1` if generated docs/templates are touched.
  - `mkdocs build --strict` if docs changes are included.
  - Final `go test ./...`, `go build ./...`, `go run . validate --json`, and
    `git diff --check`; before commit, also run `git diff --cached --check`.

## Alternatives Considered

- Approach A: Documentation and artifact-only cleanup.
  - Scope: Replace stale docs examples, compress external comparison prose, and
    normalize archived research references. Leave runtime behavior unchanged.
  - Tradeoffs: Low risk and quick, but leaves the real runtime compatibility
    layers (`--quick`, task evidence leniency, artifact version metadata) in
    place.

- Approach B: One concentrated cleanup pass with bounded runtime changes.
  - Scope: Remove `--quick`; remove unused `ManifestVersion`; remove
    `ArtifactState.Version` and tracked `version: 1` artifact entries; tighten
    task evidence parsing to the current flat evidence shape; replace stale docs
    examples; compress product docs' external comparison; normalize archived
    local upstream references. Keep still-needed migration layers.
  - Tradeoffs: Best match for the user's "second confirmation, then modify as
    much as practical in one pass" preference. Requires focused regression
    tests and careful archived fixture updates, but avoids the riskiest archive
    path-generation rewrite.

- Approach C: Broad cleanup including archive path persistence and config/policy
  strictness.
  - Scope: Approach B plus relative archived artifact paths, removal of
    `.slipway.yaml` unknown-top-level preservation, and strict advisory policy
    pack keys.
  - Tradeoffs: More complete, but mixes portability cleanup with persisted
    archive semantics and external config contract changes. This is likely too
    broad for a single safe implementation pass.

- Selected: Approach B is the recommended direction for the second confirmation.
  It removes the clearest unnecessary runtime layers and metadata while
  preserving still-needed compatibility around lifecycle logs, worktree-bound
  changes, marker-gated OpenCode cleanup, narrow JSON handoff projections, and
  run-version-bound evidence freshness.

## Unknowns

- Resolved: Should implementation be split into many separate changes? -> No.
  User clarified that after the second confirmation the preference is to modify
  as much as practical in one concentrated governed pass.
- Resolved: Are generated adapter trees source duplicates? -> No. They are
  projections from `toolgen` and capability registries; cleanup should target
  source registries and generated-surface policies, not delete generated trees.
- Resolved: Do tracked archived task evidence files require the nested
  `task_run` compatibility shape? -> No tracked evidence use was found; tracked
  task evidence uses the flat shape.
- Remaining: Whether to include archive path-generation normalization in this
  change. Recommendation: defer it unless explicitly approved, because it
  affects persisted archive semantics and existing lifecycle tests.
- Remaining: Whether to remove advisory policy-pack key aliases now.
  Recommendation: defer unless current product policy declares policy packs
  internal-only, because aliases may be an external config convenience.

## Assumptions

- The user wants research first, then a second confirmation, then a concentrated
  implementation pass. Evidence: `intent.md#Approved Summary`.
- Tracked archived evidence is a valid compatibility baseline for this repo.
  Evidence: current tracked `artifacts/changes/archived/**/evidence/tasks`
  inventory and flat JSON shape scan.
- Root-help-hidden `--quick` is not a supported public user contract. Evidence:
  `cmd/root_help_test.go:18-20` asserts root help does not contain `quick`.
- Run-version and schema-version fields used for freshness or persisted schema
  validation are not cleanup targets. Evidence:
  `internal/model/execution_summary.go:110-126`,
  `internal/model/execution_summary.go:181-186`,
  `internal/model/wave_execution.go:10-18`,
  `internal/model/governance_snapshot.go:9-29`, and
  `internal/engine/progression/evidence.go:97-108`.

## Canonical References

- `artifacts/changes/deeply-research-and-optimize-backward-compatibility-layers-redundant-layers-unnecessary-upstream-references-and-unnecessary-version-metadata/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/codebase/TESTING.md`
- `cmd/next.go`
- `cmd/run.go`
- `cmd/root_help_test.go`
- `internal/engine/progression/advance.go`
- `internal/engine/progression/advance_governed.go`
- `internal/engine/progression/wave_sync.go`
- `internal/model/types.go`
- `internal/engine/artifact/manager.go`
- `internal/state/lifecycle.go`
- `internal/state/lifecycle_event.go`
- `internal/state/store.go`
- `internal/toolgen/toolgen.go`
- `internal/toolgen/toolgen_test.go`
- `internal/model/config.go`
- `internal/engine/governance/policy_pack.go`
- `cmd/status_json.go`
- `cmd/next_handoff.go`
- `internal/engine/capability/registry.go`
- `internal/engine/capability/surfaces.go`
- `docs/design.md`
- `docs/workflow.md`
- `artifacts/changes/archived`
