# Skills Strengthening Plan

## 1. Motivation

The Skills Integration plan (B0-B8) has landed: 25 catalog skills export to
`.codex/skills/slipway/` with adapter frontmatter + `provenance.yaml`. Direct
comparison with `skills_ref/` still shows that catalog bodies retain only a
small, single-digit-percent slice of the source corpus depth: current rendered
bodies for skills such as `gha-security-review`, `root-cause-tracing`, and
`independent-review` are in the few-dozen-line range while source skills span
hundreds to thousands of lines.

Distillation keeps bodies lean *by design*, but the richness that was trimmed
was never migrated to `references/` or `scripts/`, so the catalog currently
ships without the condition-triggered material that source skills rely on.
The only already-declared hydrate owner, `context-assembly`, still points at a
non-file-backed reference shelf, so contract tightening must normalize that
owner instead of layering a second hydrate shape on top of it.

This plan is the first strengthening wave, not a claim that all 25 catalog
skills become "thick enough" in one pass. Wave 1 targets shared export/runtime
plumbing plus the 10 highest-leverage thin skills (`context-assembly`, the five
reference-heavy tracks in PR-1, and the four typed-partial tracks in PR-3).
Remaining catalog skills stay in follow-on scope after this wave's metrics and
rendered-tree evidence are reviewed.

At the current inventory size, the remaining 15 catalog skills likely require
another 2-3 strengthening waves of comparable size after Wave 1. That is a
planning estimate, not a committed schedule; the Wave-1 metrics report may
shrink or expand it.

Delivery is expected as six mergeable PRs because PR-4 is intentionally split
into PR-4a and PR-4b to keep the runtime wiring reviewable. Every PR is
independently mergeable and independently revertible. Phase ordering reflects
dependency: PR-0 unlocks PR-1/PR-2, PR-3 stays orthogonal to runtime wiring but
is still part of Wave-1 completion, PR-4a requires PR-1 output, and PR-4b
requires PR-4a plus the PR-3 boundary freeze between always-rendered and
on-demand content.

## 2. Non-goals

- No change to `ResolveNextSkill` or its contract. PR-4a may extend
  `capability.Resolve()` with additional read-only output fields, but does not
  change `Resolve()` decision semantics or governed progression authority.
- No new catalog skills. Count stays at 25.
- No change to the generated adapter frontmatter contract in
  `.codex/skills/slipway/<id>/SKILL.md`; PR-0 only widens who receives support
  files. Authoring-side `SKILL.md::hydrate_references` evolves in PR-1/PR-4a.
- No change to `skills_ref/` content.
- No claim that this wave fully restores source depth for all 25 catalog
  skills; it fixes shared plumbing and the first high-value slice only.
- No second global target-budget lift in this wave beyond PR-3's T1/T2
  adjustment. If a strengthened skill still lands in warning-band, handle that
  via explicit PR notes or `references/` rebalance, not another blanket budget
  expansion.

## 3. PR-0 - Support-file export chain (non-catalog skills)

**Goal.** Reuse the already-shared `emitSkillSupportFiles` export chain for
governance / standalone / technique skills and widen provenance emission so
non-catalog skills can ship `provenance.yaml` next to `references/` and
`scripts/` when those support payloads exist template-side.

### Files to change
- `internal/toolgen/toolgen.go`: keep the existing `emitSkillSupportFiles`
  calls in the governance / standalone / technique / catalog loops. Replace
  the caller-owned `includeProvenance bool` gate with a template-side presence
  check. For symmetry the helper may use a single "support payload exists"
  predicate (`provenance.yaml`, `references/`, or `scripts/`), but the actual
  behavior change in this PR is provenance widening for non-catalog skills.
- `internal/tmpl/templates.go`: no production code change required; add an
  enumeration test only if the template-side predicate needs explicit
  `TemplateFS()` coverage.

### Tests to add
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesForNonCatalogSkills`
  - Fixture: a technique skill template with `provenance.yaml` only.
  - Assert: rendered output contains `provenance.yaml` with `name:` matching
    the directory.
- `internal/toolgen/toolgen_test.go::TestCatalogSkillsRetainProvenanceOnPresenceCheckMigration`
  - Fixture: generate the catalog skill tree before/after the
    template-side-presence migration.
  - Assert: every catalog skill that currently ships `provenance.yaml`
    continues to ship it; PR-0 must not regress existing catalog coverage.
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesWithoutProvenanceStillCopiesReferences`
  - Fixture: a skill template with `references/` or `scripts/`, but no
    `provenance.yaml`.
  - Assert: support files are copied; `provenance.yaml` is omitted.
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesSkipsEmpty`
  - Fixture: a skill template with no support files.
  - Assert: no empty directories created; no error.
- `internal/toolgen/toolgen_test.go::TestGeneratedSkillTreeInventoryManifest`
  - Generate the `.codex/skills/slipway/` tree in a temp dir and compare a
    normalized `path -> file_kind,executable` inventory against a checked-in
    golden manifest. `executable` is asserted only on POSIX; Windows normalizes
    to a platform-stable non-exec sentinel.
  - Goal: CI catches accidental structural drift (missing support files,
    unexpected extras, executable-bit flips) without turning intentional
    content edits into perpetual hash churn. Semantic content drift stays with
    rendered-tree diff review and feature-specific fixture tests.

### Acceptance
- `init --tools codex --refresh` produces `.codex/skills/slipway/<id>/provenance.yaml`
  for every non-catalog skill that has a template-side `provenance.yaml`.
- Catalog skills that currently ship `provenance.yaml` continue to do so after
  the template-side presence-check migration.
- Existing `references/` / `scripts/` export behavior remains unchanged across
  catalog, governance, standalone, and technique skills.
- `go test ./internal/toolgen/... -count=1` passes.
- Generated skill-tree structural drift is caught by the checked-in inventory
  manifest in CI; semantic content drift remains part of rendered-tree diff
  review and targeted fixture coverage.

---

## 4. PR-1 - References for five source-rich skills + hydrate-owner normalization

**Goal.** Recover 95%+ of the selected action-changing source material as
condition-triggered reference content while keeping skill bodies within tier
budget. For this PR, "95%+" is an authoring-review metric recorded in PR notes
as a per-skill byte-ratio table (`rendered_reference_bytes /
selected_source_bytes`) plus a "rejected or collapsed source sections" log, not
a CI gate. In parallel, normalize the
already-declared `context-assembly` hydrate owner so PR-4a starts from one
canonical, file-backed contract.

### Distillation rubric
- Keep condition-triggered operational content: remediation sequences, decision
  tables, failure signatures, tool invocation recipes, and anti-patterns that
  change operator action.
- Drop or collapse narrative motivation, long examples, vendor overview, and
  prose already represented in the slim `SKILL.md` body.
- Boundary with PR-3 typed partials: always-rendered schema / checklist /
  procedure content stays in `SKILL.md` or typed partials; bulky or situational
  material goes to `references/`.
- PR-3 fallback rule: if a candidate section is not promoted into
  `PROSE.tmpl`, `CHECKLIST.tmpl`, or `VERDICT.tmpl` in the same wave, keep that
  material in `references/`; do not defer it out of both surfaces.

### New files under `internal/tmpl/templates/skills/<id>/references/`

| Skill | References | Primary sources |
|-------|-----------|------------------|
| `gha-security-review` | `pinning.md`, `oidc-trust-boundaries.md`, `self-hosted-runners.md`, `secrets-exfil-patterns.md` | getsentry/gha-security-review, trailofbits/agentic-actions-auditor |
| `root-cause-tracing` | `five-whys.md`, `parallel-hypotheses.md`, `triage-playbook.md` | superpowers/systematic-debugging, wshobson/parallel-debugging, trailofbits/debug-buttercup |
| `sast-orchestration` | `codeql-recipes.md`, `semgrep-recipes.md`, `sarif-merge.md` | trailofbits/codeql, trailofbits/semgrep, trailofbits/sarif-parsing, trailofbits/audit-augmentation |
| `supply-chain-audit` | `sbom-checklist.md`, `typosquat-patterns.md`, `transitive-pinning.md` | trailofbits/supply-chain-risk-auditor, alirezarezvani/dependency-auditor |
| `incident-response` | `severity-matrix.md`, `comms-template.md`, `postmortem-outline.md` | alirezarezvani/incident-commander + incident-response, sickn33/acceptance-orchestrator |

### Existing owner normalization
- Materialize `internal/tmpl/templates/skills/context-assembly/references/codebase-map.md`
  so the repository's already-declared hydrate owner stops pointing at a
  non-file-backed shelf before PR-4a compare gates land. This is contract
  normalization, not part of the five source-depth recovery tracks above.

### Other code changes
- Each affected `SKILL.md`: for the five newly hydrated skills, adopt the same
  descriptive `hydrate_references:` record shape (`name`, `reason`) already
  used by `context-assembly`, so authoring/export metadata shares one canonical
  form before any registry compare gate exists.
- Until PR-4a lands, no registry-backed compare gate protects
  `hydrate_references:`. The only temporary guardrails are
  `TestHydrateReferencesResolveToFiles` plus provenance coverage updates.
  `TestHydrateReferencesResolveToFiles` is the temporary frontmatter-only gate
  for the PR-1 -> PR-4a window: it enforces typed record shape, unique names,
  and file-backed resolution without depending on registry metadata. That drift
  window is acceptable because no runtime surface consumes the field before
  PR-4a; PR-1 closes the existing `context-assembly` dangling-owner gap rather
  than widening it. After PR-4a lands, keep the same test as the orthogonal
  file-existence gate; it stops being the only hydrate contract check, but it
  remains the only one that proves file-backed resolution.
- Each affected `provenance.yaml`: extend `inputs:` so each new reference
  points at its source lines, including the normalized `context-assembly`
  owner whose reference becomes file-backed in this PR.
- `context-assembly/references/codebase-map.md` remains method-first in PR-1.
  It may not depend on the CLI, flags, or output contract of
  a future helper script. If authoring needs executable guidance in this wave,
  point at the already-shipped `slipway codebase-map` command instead of
  introducing a second context-assembly entrypoint.
- PR-1 closeout note must state: the next PR allowed to mutate
  `hydrate_references:` frontmatter is `PR-4a`. PR-2 and PR-3 may consume the
  field read-only, but may not reshape it.

### Tests to add
- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences`
  - Input: the 5 skill IDs above.
  - Assert: rendered `.codex/skills/slipway/<id>/references/` non-empty;
    every `.md` starts with `# `.
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles`
  - Input: every catalog skill whose `SKILL.md` declares
    `hydrate_references:`.
  - Assert: each listed entry is an object, `name` is non-empty and unique
    within the skill, `reason` is non-empty when present, `name` is a basename
    with no path separators or `..`, and `name` resolves to an existing `.md`
    file under the skill's `references/` directory.
- Body size: reuse
  `internal/engine/capability/gates_test.go::TestSizeBudgetsForRegisteredSkills`;
  that gate only bounds `SKILL.md`, not references.
- Reference size: add
  `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget` enforcing
  <= 24 KB per reference file and <= 64 KB total per skill. The total cap is a
  byte budget, not a file-count ceiling; it still leaves room for multiple
  medium-size references while forcing real distillation instead of creating a
  second body beside `SKILL.md`.

### Acceptance
- Each reference file <= 24 KB; total references per skill <= 64 KB (enforced
  by `TestReferenceFileSizeBudget`).
- Rendered-tree diff shows new `references/` directories and expanded
  `provenance.yaml inputs:` coverage for the five source-rich skills plus the
  normalized `context-assembly/codebase-map.md` owner.
- Manual review rule: no reference file may reproduce >=50% of the owning
  `SKILL.md` body line-for-line. This remains rendered-tree review guidance,
  not a CI gate.
- PR notes include the per-skill source-depth byte-ratio table used to support
  the 95%+ recovery claim for this first wave, plus a short log of source
  sections intentionally rejected or collapsed.
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  passes.

---

## 5. PR-2 - Deterministic scripts

**Goal.** Ship executable actions that source skills describe in prose, so
agents can invoke them directly.

### New files

| Script | Owning skill | Purpose |
|--------|-------------|---------|
| `scripts/find-polluter-go.sh` | `root-cause-tracing` | Bisect test-order pollution for Go test suites. |
| `scripts/merge-sarif.sh`   | `sast-orchestration` | `jq`-based multi-SARIF aggregator. |
| `scripts/pin-actions.sh`   | `gha-security-review`| Rewrite `uses: foo@v1` -> `uses: foo@<sha>` using a supplied checked-in mapping, never live network lookups. |

### Code changes
- `internal/toolgen/toolgen.go`: no permission-plumbing refactor is expected
  while helper files stay `.sh`; `writeDeterministic` already writes `.sh`
  outputs with `0o755`. Keep this PR focused on adding scripts plus regression
  tests, and only introduce explicit chmod logic if a future helper needs
  executable mode without a `.sh` suffix.
- `pin-actions.sh` must stay offline and deterministic. It consumes an explicit
  checked-in mapping fixture or manifest passed explicitly at runtime
  (for example `--mapping <path>`); unresolved refs produce a stable report and
  non-zero exit. No GitHub API, `gh`, or tag-resolution network calls are
  allowed in PR-2.
- `pin-actions.sh` does not ship with a universal Slipway-owned tag -> SHA
  database. The caller repo owns the checked-in mapping file; Slipway ships an
  example schema/fixture and the usage contract.
- `pin-actions.sh` is a deterministic rewrite helper, not a universal
  turn-key pinning service. When no checked-in mapping exists, the references
  remain the primary guidance surface and the script must fail explicitly
  instead of pretending to be generally actionable.
- Script naming should stay language-specific where helper behavior is
  language-bound. `root-cause-tracing` remains language-neutral in prose; the
  Go-specific helper is only one concrete recipe. Reserve
  `scripts/find-polluter-<lang>.sh` as the namespace for future language
  variants so the first Go helper is not a one-off special case.
- `context-assembly` reuses the already-shipped `slipway codebase-map` command
  in this wave. PR-2 does not add `scripts/codebase-map.sh`; avoid shipping a
  second repo-mapping entrypoint until there is a demonstrated gap in the
  command surface.
- Scripts with external dependencies must fail fast with actionable messages
  when prerequisites are missing (for example `jq` for `merge-sarif.sh`).
- CI must declare the non-POSIX script dependencies used by fixture-contract
  tests. For this wave that means `jq`: local runs may skip the fixture test
  with an explicit missing-tool message, but CI must install it and execute the
  contract path rather than silently skipping.
- Wave-1 helper scripts are POSIX-only. The owning `SKILL.md` / `references/`
  must label them as optional POSIX helpers and preserve a prose fallback for
  non-POSIX agents instead of implying universal executability.

### Tests to add
- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit`
  - Assert: rendered `scripts/*.sh` stat shows `0o111` bits.
- `internal/toolgen/toolgen_test.go::TestScriptSyntaxCheck`
  - Invoke `bash -n <script>` for each rendered script; fail on non-zero.
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`
  - `merge-sarif.sh`: merge fixture SARIF inputs and assert deterministic
    output shape.
  - `pin-actions.sh`: rewrite a fixture workflow from tag refs to SHAs using a
    checked-in mapping fixture; assert unresolved refs fail without network
    fallback and missing `--mapping` fails with actionable usage text.
  - `find-polluter-go.sh`: at minimum, assert stable usage / dependency errors
    when required inputs are absent.

### Acceptance
- All three scripts pass `bash -n`.
- Executable bit present on POSIX filesystems (skip assertion on Windows).
- Each script has at least one deterministic fixture or failure-contract test;
  PR-2 does not rely on syntax-only coverage.
- `init --tools codex --refresh` writes the scripts into the generated skill
  tree.

---

## 6. PR-3 - Extend typed partials + lift target budgets

**Goal.** Grow body depth on four source-rich skills by extending the
*already-landed* typed-template assembler (`renderCatalogSkill` in
`internal/toolgen/toolgen.go`), and lift T1/T2 target budgets without touching
the existing hard-max + `size_rationale` rule.

### Baseline already in tree
- Assembler order `SKILL.md` -> `PROSE.tmpl` -> `CHECKLIST.tmpl` ->
  `VERDICT.tmpl` is implemented in `renderCatalogSkill` and covered by
  `internal/toolgen/toolgen_test.go`.
- `independent-review` already ships `PROSE.tmpl` + `CHECKLIST.tmpl` +
  `VERDICT.tmpl` and is the reference example; PR-3 does not touch it.

### Typed templates to extend

| Skill | Partial(s) to add | Purpose |
|-------|-------------------|---------|
| `spec-trace` | `CHECKLIST.tmpl` | spec-to-code coverage matrix |
| `threat-modeling` | `PROSE.tmpl`, `CHECKLIST.tmpl` | STRIDE prose + asset table |
| `coverage-analysis` | `VERDICT.tmpl` | gap-report schema |
| `security-review` | `CHECKLIST.tmpl` | merge of insecure-defaults + sharp-edges items |

### Boundary with references
- Typed partials are always assembled when their attachment mode applies.
- `references/` stays file-backed and on-demand; it does not replace required
  checklist / report-schema / procedure sections.
- `security-review`'s `CHECKLIST.tmpl` is intentionally always-rendered: the
  checklist is part of the normal review attachment surface, not conditional
  reference material.

### Budget policy update
- Update target budgets in schema docs, distillation checklists, the
  2026-04-11 plan/delivery pair (EN + zh-CN), and capability size gates:
  - T1: 2 KB -> 2.5 KB
  - T2: 3 KB -> 3.5 KB
  - T3: unchanged at 1.5 KB (T3 skills stay lean; body strengthening for T3
    ships via references, not via the target lift).
- Keep hard-max bands at 6 / 8 / 3 KB and keep the existing
  `size_rationale` requirement *only above hard-max*. PR-3 does not promote
  `size_rationale` into the warning band and does not introduce a new
  runtime warn/critical surface.
- `security-review` is a T1 skill, so the immediate pressure case in this wave
  is the 2.5 KB T1 target, not the T2 band. If its new `CHECKLIST.tmpl` still
  leaves it in warning-band, do not pre-raise tier budgets again in this wave;
  keep the warning explicit or move overflow to `references/`.

### Tests to add
- `internal/engine/capability/gates_test.go`
  - Update `tierSizeBudget` and `TestSizeBudgetsForRegisteredSkills` for the
    lifted target values. The existing warning-band `t.Logf` stays a log,
    not an error.
  - `TestTierSizeBudgetRequiresRationaleAboveHardMax` stays unchanged in
    intent: rationale is still only required above hard-max.
- `internal/toolgen/toolgen_test.go::TestTypedPartsRendered`
  - For each of the 4 skills above, assert the CHECKLIST / PROSE / VERDICT
    section headings appear in the assembled `SKILL.md`.

### Acceptance
- Capability size / schema / binding / provenance gates remain green after the
  target lift.
- No skill exceeds hard-max without `size_rationale`. Warning-band skills are
  logged but not blocked, matching the existing gate semantics.
- Record the rendered byte size of `security-review`, `spec-trace`,
  `threat-modeling`, and `coverage-analysis` once their typed partials land; if
  any still live in the warning band, that outcome must be explicit in the PR
  notes rather than assumed away by the target lift.

---

## 7. PR-4 - Hydrate wiring across three selection paths

**Goal.** Turn static `references/` into condition-triggered hydration. Three
distinct selection paths already exist in the codebase and PR-4 must wire
hydrate through all three, not only through auto-route.

### Selection paths (existing, not invented by this plan)
1. **Auto-route** - `capability.Resolve()` -> `pickRoute()` fires only for
   `BindingCommandAuto` / `BindingCommandView` bindings. Surfaced today via
   `resolveEffectiveRouteMode` / `resolveEffectiveRouteView`.
2. **Manual explicit** - user passes `--mode=<skill-id>` or
   `--view=<skill-id>`; `validateRouteMode` / `validateRouteView` accept it.
   For read-only surfaces, `ValidViewsForCommand` admits both
   `BindingCommandManual` and `BindingCommandView`. This path preserves the
   validated explicit skill id and does not depend on `pickRoute()`.
3. **Support / host-hint** - `next` renders technique hints from
   `resolution.Supports` via `appendCatalogHints`. Not a routed surface.

First-wave skills exercise all three paths, though some participate in more
than one: `gha-security-review`, `sast-orchestration`, and
`supply-chain-audit` cover the manual-explicit path; `root-cause-tracing`
first hydrates on `next` through the support / host-hint path while its
existing `repair` auto-route binding remains unchanged; `incident-response` is
the primary `status` / `health` command-default view case in this wave. In
current shipped code that route is chosen from command context; the
change-selected surface only determines whether auto-routing is consulted.

### Delivery split
- **PR-4a** - populate hydrate references on all three paths and surface them
  read-only on affected commands. No `--hydrate` expansion yet.
- **PR-4b** - add `--hydrate` expansion only after PR-4a output is stable.

### Authority and compare gate (single source of truth)
- Registry stays the runtime authority for binding metadata and, after PR-4a,
  for hydrate metadata as well. Keep the canonical authoring/export shape
  aligned with the already-live `context-assembly` owner by promoting a typed
  `Skill.HydrateReferences []HydrateReference` record (at minimum `Name`,
  `Reason`) rather than collapsing to bare strings.
- Frontmatter `hydrate_references:` becomes the export mirror of those typed
  records.
- `HydrateReference.Name` stays the authoring-time basename inside a skill, but
  runtime outputs and equality keys use a collision-safe skill-relative form
  `<skill-id>/<name>`. Deduping and sorting happen on that full key, never on
  basename alone.
- Add `TestHydrateReferencesMirrorRegistry` in
  `internal/engine/capability/gates_test.go`, modeled on the existing
  binding-compare gate (`TestFrontmatterMirrorsRegistryBindings`): for every
  catalog skill, require `SKILL.md::hydrate_references` record-equal to
  `registry.HydrateReferences` after sorting by `name`. This is the missing
  1:1 contract.
- Keep `TestHydrateReferencesResolveToFiles` after PR-4a as an orthogonal
  file-backed resolution gate. Mirror-registry equality proves contract sync;
  it does not prove that the referenced file still exists on disk.

### Code changes
- `internal/engine/capability/`
  - Add a `HydrateReference` struct and
    `Skill.HydrateReferences []HydrateReference`. Seed values from the
    canonical SKILL.md frontmatter normalized in PR-1, including
    `context-assembly`.
  - Populate `Resolution.HydrateReferences` inside `Resolve()` whenever a
    route or support is emitted: union across the selected skill plus any
    `Supports` entries, flattened to stable-sorted and deduped
    skill-relative keys (`<skill-id>/<name>`). Resolver stays read-only with
    respect to the kernel.
  - Export a helper `capability.HydrateReferenceKeysForSkill(reg, skillID)` so
    the manual-explicit path and `next` hint rendering can look up collision-
    safe keys by skill id without going through `Resolve()`.
  - Update resolver tests so only `llm_tiebreak` remains reserved after
    PR-4a; hydrate assertions become real for the auto-route cases.
  - Add a routing-invariant test over a representative signal matrix: PR-4a may
    populate `HydrateReferences`, but it must not change the pre-existing
    `Route` or `Supports` outputs for already-green bindings.
- `cmd/route_flags.go`
  - Add `resolveEffectiveRouteHydrate(command, explicitMode, signals...)` and
    `resolveEffectiveViewHydrate(...)`. Precedence: when `explicit != ""`
    and validation passed, look up via `HydrateReferenceKeysForSkill`; if that
    explicit skill has zero hydrate refs, return an empty slice and do not fall
    back to `Resolve()`. Only the empty-explicit path falls back to resolver
    output. This is the fix for the manual-explicit gap.
- `cmd/hydrate_render.go`
  - Add one shared hydrate-render helper for both text and JSON surfaces.
    `review`, `status`, `health`, `validate`, and `next` must reuse it instead
    of open-coding `Hydrate:` formatting, JSON field population, ordering, or
    empty-slice elision independently. PR-4a should leave one canonical place
    for delimiter, sort, and future size-cap behavior.
- `cmd/next_skill_view.go`, `cmd/next.go`
  - Extend `appendCatalogHints` and `techniqueHint` so each support hint can
    carry `hydrate_references[]`; `next --json` keeps those hydrate keys nested
    under each `technique_hint` object and does not emit a top-level merged
    hydrate field for support-path output. `cmd/next.go` renders the same keys
    as a stable indented `Hydrate:` line under the owning hint instead of
    inventing a separate top-level `next` surface.
- `cmd/review.go`, `cmd/status.go`, `cmd/health.go`
  - Render a `Hydrate:` line (text) and field (JSON) when the effective
    resolution for this surface returns non-empty references. JSON shape:
    `"hydrate_references": ["gha-security-review/pinning.md", ...]`. Text
    shape (one line):
    `Hydrate: gha-security-review/pinning.md, gha-security-review/oidc-trust-boundaries.md`.
- `cmd/validate.go`
  - Keep PR-4a scoped to JSON output for `validate`; it currently exposes a
    JSON-only surface, so this wave adds `"hydrate_references": [...]` there
    without inventing a new text renderer.
- `cmd/review.go`, `cmd/status.go`, `cmd/health.go`
  - PR-4b: add `--hydrate` that prints each selected reference verbatim,
    one file per section, preceded by
    `===== SLIPWAY HYDRATE: <skill-id>/<name> =====` so golden tests can pin a
    stable delimiter with low prose-collision risk. The helper validates that
    rendered hydrate keys do not contain `=` or newlines before rendering.
    Total emitted hydrate body is capped at 32 KB per invocation; above that,
    the command fails deterministically with the selected keys and byte estimate
    instead of dumping oversized context into the consumer agent.

### First-wave binding table (with selection path)

| Skill | Binding type | Selection path | Initial hydrated refs | First surfaced in |
|-------|--------------|----------------|-----------------------|-------------------|
| `gha-security-review` | CommandManual on `review`, `repair` | Manual explicit via `--mode=gha-security-review` | `pinning.md`, `oidc-trust-boundaries.md`, `self-hosted-runners.md`, `secrets-exfil-patterns.md` | `review` |
| `root-cause-tracing` | CommandAuto on `repair`; TechniqueHint / HostEmbedded on `wave-orchestration` | First hydrated output lands through support / host-hint via `appendCatalogHints`; existing `repair` auto-route binding remains unchanged in this wave | `five-whys.md`, `parallel-hypotheses.md`, `triage-playbook.md` | `next` (wave-orchestration host); `repair` remains existing, unchanged |
| `sast-orchestration` | CommandManual on `review`, `validate`, `repair` | Manual explicit via `--mode=sast-orchestration` | `codeql-recipes.md`, `semgrep-recipes.md`, `sarif-merge.md` | `review`, `validate` |
| `supply-chain-audit` | CommandManual on `review`, `repair`, `status` | Manual explicit via `--mode` on `review` / `repair` and `--view` on `status` (read-only surfaces admit manual bindings through `ValidViewsForCommand`) | `sbom-checklist.md`, `typosquat-patterns.md`, `transitive-pinning.md` | `status`, `review` |
| `incident-response` | CommandView on `status`, `health` | Command-default auto view via `pickRoute` on change-selected `status` / `health` surfaces | `severity-matrix.md`, `comms-template.md`, `postmortem-outline.md` | change-selected `status`, `health` |

Verify each row's current binding set against the registry constructors
(`sastOrchestration`, `ghaSecurityReview`, `supplyChainAudit`,
`rootCauseTracing`, `incidentResponse`) before authoring tests; if a row's
binding differs, fix the row, not the registry (registry is authority).

### Tests to add
- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  - For every catalog skill: frontmatter `hydrate_references:` equals
    `Skill.HydrateReferences` as sorted-by-name typed records.
- `internal/engine/capability/resolver_test.go`
  - Auto-route case: `incident-response` on `status` view emits
    `incident-response/severity-matrix.md` via `Resolution.HydrateReferences`.
  - Support case: `wave-orchestration` host emits `root-cause-tracing` refs
    through the support path.
  - Stability: emitted references are sorted and deduped.
  - Invariant: representative routed cases preserve pre-PR-4a `Route` and
    `Supports` outputs exactly; only `HydrateReferences` is newly populated.
- `cmd/route_flags_test.go`
  - `resolveEffectiveRouteHydrate` returns expected refs for
    `--mode=gha-security-review` and `--mode=sast-orchestration` without going
    through `Resolve()`.
  - `resolveEffectiveViewHydrate` returns expected refs for
    `--view=supply-chain-audit` and `--view=incident-response` on read-only
    surfaces without going through `Resolve()`.
  - Explicit skill with zero hydrate refs returns an empty slice and does not
    fall back to resolver auto-route output.
- `cmd/hydrate_view_test.go` (new, naming mirrors `route_flags_test.go`)
  - Golden: `review --mode=gha-security-review` lists
    `gha-security-review/pinning.md`.
  - Golden: `status --view=incident-response` lists
    `incident-response/severity-matrix.md`.
  - Golden: `next` on a `wave-orchestration` host lists
    `root-cause-tracing/five-whys.md` in the support hint's
    `hydrate_references[]` JSON field and as an indented `Hydrate:` line in the
    technique-hint block.
  - Golden: the `next` hint block render order stays stable: pre-existing
    technique hints remain in place, appended support hints keep deterministic
    order, and hydrate keys within each hint are stable-sorted.
- `cmd/hydrate_flag_test.go` (new, PR-4b only)
  - `--hydrate` output contains each reference H1 and the
    `===== SLIPWAY HYDRATE: <skill-id>/<name> =====` delimiter.
  - Oversize case: when selected hydrate bodies exceed 32 KB total, the command
    fails with a stable `hydrate_output_too_large` contract instead of emitting
    partial or truncated bodies.

### Acceptance
- Manual-explicit: `review --mode=gha-security-review`,
  `validate --mode=sast-orchestration`, `status --view=supply-chain-audit`
  all surface hydrate keys (PR-4a); `validate` does so in JSON output only.
- Auto-route: change-selected `status` / `health` surfaces continue to expose
  the existing command-default `incident-response` route; PR-4a adds hydrate
  refs to that route but does not claim new change-derived routing signals.
  Diagnostics-only output continues to preserve only explicit `--view`.
- Support / host-hint: `next` on a `wave-orchestration` host surfaces
  `root-cause-tracing` refs inside the owning technique-hint block, with
  `hydrate_references[]` nested per hint in JSON and stable render order in
  text.
- `--hydrate` prints the selected reference bodies verbatim when the selected
  body total is <= 32 KB; above that it fails deterministically with explicit
  keys and size diagnostics instead of dumping oversized context (PR-4b).
- `TestHydrateReferencesMirrorRegistry` passes; registry stays single
  authority.
- No regression in existing `cmd/...` / `capability` golden tests.

---

## 8. Execution order and gates

1. **PR-0 first.** Smallest blast radius; every later PR depends on its
   support-file pipeline.
2. **PR-1 and PR-2 in parallel.** References and scripts are independent.
   During that window, `hydrate_references:` frontmatter may change only in
   PR-1; the next PR allowed to mutate it is `PR-4a`.
3. **PR-3 stays orthogonal but is still Wave-1-required.** Land it before
   `PR-4b` so the always-rendered vs on-demand boundary is frozen before
   hydrate expansion UX is finalized.
4. **PR-4a after PR-1.** `PR-4b` follows only after `PR-4a` output and tests
   are stable.
5. Each PR runs three hard gates before merge:
   - `go test ./... -count=1`
   - `init --tools codex --refresh` on this repo, diff review on the resulting
     `.codex/skills/slipway/` tree. The checked-in inventory manifest catches
     accidental structural drift automatically; manual diff review remains
     intentionally required for semantic inspection.
  - Phase-appropriate command smoke checks (`next --preview --json`,
    `review --json`, `validate --json`, explicit manual-view checks
    (`status --json --view <id>`,
    `health --json --governance --view <id>`), and change-selected default-route
    checks (`status --json --change <slug>`,
    `health --json --governance --change <slug>`) show the expected routed /
    hydrate surfaces with no unexpected regressions.
6. Every PR that mutates this plan family updates both the English and
   `zh-CN` files in the same batch; Wave-1 does not allow translation drift.
7. **Wave-2 trigger is explicit.** Within 7 days of Wave-1 merge, produce a
   short metrics report covering rendered reference byte ratios, rejected-source
   logs, warning-band skills, hydrate smoke results, and any support-file drift.
   Wave-2 scope is chosen only after that report is reviewed alongside the
   rendered-tree evidence.

## 9. Out of scope

- Claude adapter export refresh (tracked separately once Codex adapter
  stabilizes).
- Rewriting `skills_ref/` provenance; this plan only adds pointers into it.
- Deferred source `alirezarezvani/prompt-governance` remains deferred.
