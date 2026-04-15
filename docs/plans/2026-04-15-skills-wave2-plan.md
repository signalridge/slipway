# Skills Strengthening Plan — Wave 2 (Draft)

**Status.** Draft, awaiting the Wave-1 metrics report required by
`2026-04-14-skills-strengthening-plan.md` §8.7. Do not open implementation
PRs for this wave until that report is reviewed. Per-skill byte targets,
reference inventories, and hydrate binding rows below are provisional and
must be re-validated against Wave-1 metrics before the first Wave-2 PR
lands.
Wave-1 measurements already put `gha-security-review` at `63,645 / 65,536`
reference bytes (~97.1% of cap). If any later wave reopens that skill, it
must budget collapse/defer work before adding material.

## 1. Motivation

Wave-1 shipped the shared plumbing: PR-0 support-file export for non-catalog
skills, PR-4a/4b hydrate wiring across three selection paths, and a T1/T2
target budget lift. Wave-2 is the first pattern-application wave. It applies
the same treatment (references + optional scripts + optional typed partials +
hydrate wiring) to five catalog skills whose upstream sources share a
parallel `references/` or `resources/` shelf:

- Four Trail of Bits analysis skills (`differential-review`,
  `variant-analysis`, `property-testing`, `mutation-testing`). Each has a
  well-bounded upstream reference shelf, so reference templates and fixture
  contracts are naturally reusable across them.
- `performance-profiling`, whose upstream
  (`alirezarezvani/performance-profiler` + `wshobson/distributed-tracing`)
  ships one reference plus one Python helper. Its treatment mirrors the
  Trail of Bits pattern even though the source is not Trail of Bits.

This wave does not introduce new infrastructure. It is a scope-bounded
application of Wave-1 contracts to a single family.

## 2. Non-goals

- No change to `ResolveNextSkill` or `capability.Resolve()` decision
  semantics. The hydrate wiring added here reuses Wave-1 PR-4a fields.
- No new catalog skills. Count stays at 25.
- No new typed partials (`PROSE.tmpl` / `CHECKLIST.tmpl` / `VERDICT.tmpl`)
  in Wave-2. The five skills in this wave are reference-heavy; their bodies
  are already within T2 target after Wave-1's budget lift. If a later review
  finds a typed-partial case for any of them, open a follow-up; do not
  bundle it into Wave-2.
- No further tier-budget lift. T1 stays at 2.5 KB, T2 at 3.5 KB, T3 at
  1.5 KB. Skills that land in warning-band after this wave should resolve
  via `references/` rebalance, not another budget bump.
- No `skills_ref/` rewrite. Wave-2 only adds pointers into it.

## 3. PR-1 — References for five skills

**Goal.** Recover upstream reference shelves as condition-triggered
reference content, under the same distillation rubric as Wave-1 PR-1:
condition-triggered operational content stays in `references/`; narrative
and long examples are collapsed or dropped; source-aligned filenames where
practical.

### Planned references

| Skill | Planned `references/` | Source sections mapped | Curator synthesis (if any) | Notes |
|-------|------------------------|------------------------|----------------------------|-------|
| `differential-review` | `methodology.md`, `adversarial-comparison.md`, `patterns.md`, `reporting.md` | `trailofbits/differential-review/{methodology,adversarial,patterns,reporting}.md` | Rename `adversarial.md` → `adversarial-comparison.md` to disambiguate from future adversarial-review material; log rename in provenance. | Upstream files are flat (not under `references/`); authoring must read from the source root, not assume a `references/` subdir. |
| `variant-analysis` | `methodology.md`, `codeql-variant-queries.md`, `semgrep-variant-rules.md`, `variant-report-template.md` | `trailofbits/variant-analysis/METHODOLOGY.md`; `resources/codeql/*`; `resources/semgrep/*`; `resources/variant-report-template.md` | `codeql-variant-queries.md` and `semgrep-variant-rules.md` are per-tool digests of the `resources/codeql` and `resources/semgrep` subtrees; record "multi-file digest" in provenance. | Overlaps with Wave-1 `sast-orchestration/{codeql-*,semgrep-*}.md`. Cross-reference both, do not duplicate CodeQL or Semgrep foundations — point at the sast-orchestration refs for shared material. |
| `property-testing` | `design.md`, `generating.md`, `strategies.md`, `libraries.md`, `interpreting-failures.md`, `refactoring.md`, `reviewing.md` | `trailofbits/property-based-testing/references/*` (7 files, 1:1) | None | Source is already source-aligned; keep names verbatim. |
| `mutation-testing` | `optimization-strategies.md`, `configuration.md` | `trailofbits/mutation-testing/references/optimization-strategies.md`; `trailofbits/mutation-testing/workflows/configuration.md` | None | Workflow content collapsed into a sibling reference; log the flat-map in provenance. |
| `performance-profiling` | `profiling-recipes.md`, `distributed-tracing-playbook.md` | `alirezarezvani/performance-profiler/references/profiling-recipes.md`; curated synthesis from `wshobson/distributed-tracing/SKILL.md` | `distributed-tracing-playbook.md` is curator-authored (no upstream reference file); record reason: upstream ships only SKILL.md and its tracing material does not belong in the body. | Keeps the body focused on profiling workflow while tracing material becomes on-demand. |

### Cross-wave overlap handling

- `variant-analysis` vs `sast-orchestration` (Wave-1): both reference CodeQL
  and Semgrep content. Wave-2 variant-analysis references must link into
  the existing Wave-1 `sast-orchestration/codeql-*.md` and
  `sast-orchestration/semgrep-*.md` for foundational content, and only
  restate what is specific to variant discovery / ruleset evolution. No
  line-for-line duplication. PR notes must list which sast-orchestration
  references are linked.
- If linking across skills would require a runtime feature (e.g.,
  cross-skill hydrate references), stop and escalate — Wave-2 does not
  introduce such a feature. Manual prose cross-reference inside the
  reference body is the intended mechanism.

### Code changes

- `internal/tmpl/templates/skills/<id>/references/*.md` — new files per
  the table above.
- `internal/tmpl/templates/skills/<id>/provenance.yaml` — extend `inputs:`
  to cover each new reference and any curator synthesis.
- `internal/tmpl/templates/skills/<id>/SKILL.md` — add
  `hydrate_references:` frontmatter in the typed record shape introduced by
  Wave-1 PR-1 (`name`, `reason`). Do not reshape the frontmatter contract.
- `internal/engine/capability/registry_b4.go` — populate
  `Skill.HydrateReferences` for the four affected b4 skills. Pattern mirrors
  Wave-1 PR-4a.

### Tests to add / extend

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` — extend
  input list to include the five Wave-2 skill IDs.
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles` —
  extends automatically by virtue of registry growth; assert the five new
  skills resolve.
- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  (added in Wave-1 PR-4a) — extends automatically.
- `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget` — caps
  remain 24 KB / file, 64 KB / skill total.

### Acceptance

- Each reference file ≤ 24 KB; total references per skill ≤ 64 KB.
- Rendered-tree diff shows the new `references/` directories and expanded
  `provenance.yaml inputs:` coverage for all five skills.
- PR notes include the per-skill source-depth byte-ratio table
  (`rendered_reference_bytes / selected_source_bytes`) against named source
  sections, plus a "mapped / collapsed / deferred" log.
- No reference file reproduces ≥ 50 % of the owning `SKILL.md` body
  line-for-line (manual review rule, same as Wave-1).
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  passes.

## 4. PR-2 — Scripts

**Goal.** Ship two executable helpers that upstream sources describe in
prose or ship as Python.

| Script | Owning skill | Purpose |
|--------|--------------|---------|
| `scripts/profiling-recipes.py` | `performance-profiling` | Narrowed lift of `alirezarezvani/performance-profiler/scripts/performance_profiler.py`. Input: target process / binary + profile mode; output: deterministic recipe invocation with per-platform fallback errors. Python runtime contract per Wave-1 PR-2 (fail fast on missing `python3`). |
| `scripts/find-variant.sh` | `variant-analysis` | Offline helper: given a seed finding (vulnerable pattern + file:line), emit a stable CodeQL + Semgrep query template skeleton pinned to the existing `sast-orchestration` ruleset names. No network, no tag resolution. |

### Constraints

- Reuse Wave-1 PR-2 contracts verbatim: `.sh` helpers use `0o755`, shell
  scripts pass `bash -n`, Python helpers pass `python3 -m py_compile`,
  missing-runtime paths fail with actionable messages. No new export
  plumbing.
- `profiling-recipes.py` is a narrowed lift, not a from-scratch rewrite.
  Provenance must state the lift source. Any optional third-party
  dependency must be declared in-script (for example via `uv` script
  metadata) or fail with an actionable message.
- `find-variant.sh` must not embed any specific vulnerability fixture.
  Its template output is a skeleton, not a shipping query. A reference
  file (`variant-analysis/codeql-variant-queries.md` or
  `semgrep-variant-rules.md`) must document how to complete the skeleton.

### Tests to add

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`:
  - `profiling-recipes.py`: given fixture profile mode, assert stable
    recipe output shape and that missing-runtime on unsupported platform
    fails with a stable error string.
  - `find-variant.sh`: given fixture seed, assert output contains stable
    query placeholders and that missing `--seed` fails with usage text.

### Acceptance

- Both scripts pass static checks and at least one fixture or
  failure-contract test.
- `init --tools codex --refresh` writes the scripts into the generated
  skill tree.

## 5. PR-3 — Hydrate wiring on affected selection paths

**Goal.** Populate hydrate references for the five Wave-2 skills on the
selection paths each skill actually participates in. No new infrastructure;
only registry population and surface rendering additions using helpers added
in Wave-1 PR-4a / PR-4b.

### First-wave binding table (tentative)

Verify each row against the b4 registry constructors before authoring
tests; registry is authority.

| Skill | Existing bindings (to be re-verified) | Selection path | Initial hydrated refs | First surfaced in |
|-------|----------------------------------------|----------------|-----------------------|-------------------|
| `differential-review` | TBD (`review`?) | Manual explicit via `--mode=differential-review` | `methodology.md`, `adversarial-comparison.md`, `patterns.md`, `reporting.md` | `review` |
| `variant-analysis` | TBD (`review`?) | Manual explicit via `--mode=variant-analysis` | `methodology.md`, `codeql-variant-queries.md`, `semgrep-variant-rules.md`, `variant-report-template.md` | `review` |
| `property-testing` | TBD (`validate`? `review`?) | Manual explicit via `--mode=property-testing` | `design.md`, `generating.md`, `strategies.md`, `libraries.md`, `interpreting-failures.md`, `refactoring.md`, `reviewing.md` | `validate`, `review` |
| `mutation-testing` | TBD (`validate`?) | Manual explicit via `--mode=mutation-testing` | `optimization-strategies.md`, `configuration.md` | `validate` |
| `performance-profiling` | TBD (`review`? `status`?) | Depends on binding; likely manual explicit via `--mode` on `review` and `--view` on `status` | `profiling-recipes.md`, `distributed-tracing-playbook.md` | `review`, `status` |

**Re-verification instruction.** Before writing tests, open
`registry_b4.go` and read the actual `Bindings:` slice for each skill.
Fix the row, not the registry. If no binding surface matches the
intended first-wave hydrate surface, flag it in PR notes; do not invent a
new binding just to carry hydrate refs.

### Code changes

- `internal/engine/capability/registry_b4.go` — populate
  `Skill.HydrateReferences` per row.
- No changes in `cmd/route_flags.go`, `cmd/hydrate_render.go`,
  `cmd/review.go`, `cmd/validate.go`, `cmd/status.go`, `cmd/health.go`,
  `cmd/next.go`: Wave-1 PR-4a already renders hydrate whenever the
  resolver / manual-explicit lookup returns a non-empty slice. Wave-2
  only changes what the lookup returns.
- 32 KB hydrate output cap from Wave-1 PR-4b applies. `property-testing`
  with 7 refs is the closest to the cap; check total byte estimate in PR
  notes. If it exceeds 32 KB, split the skill into reference tiers (e.g.,
  keep `interpreting-failures.md` and `reviewing.md` out of the default
  hydrate set by listing only primary refs in `hydrate_references:` and
  letting the others remain file-backed on-demand reading).

### Tests to add

- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  — extends automatically.
- `internal/engine/capability/resolver_test.go` — add cases for each new
  manual-explicit binding.
- `cmd/hydrate_view_test.go` — golden cases:
  - `review --mode=differential-review` lists
    `differential-review/methodology.md`.
  - `validate --mode=property-testing` lists
    `property-testing/design.md`.
  - `status --view=performance-profiling` (if binding exists) lists
    `performance-profiling/profiling-recipes.md`.

### Acceptance

- All five skills surface hydrate keys on at least one command surface
  (manual explicit path at minimum; change-selected or support paths if
  the existing bindings already admit them).
- Per-skill selected hydrate body total ≤ 32 KB per invocation.
- No regression in existing `cmd/...` / `capability` golden tests.

## 6. Execution order and gates

1. **PR-1 first.** References + hydrate frontmatter + registry records.
   This is the one PR that actually changes authoring surface.
2. **PR-2 and PR-3 may run in parallel after PR-1.** Scripts and hydrate
   wiring are orthogonal; both consume PR-1 output but not each other.
3. Each PR runs the same three hard gates as Wave-1 §8.5:
   - `go test ./... -count=1`
   - `init --tools codex --refresh` on this repo, diff review on the
     resulting `.codex/skills/slipway/` tree.
   - Command smoke checks for the affected surfaces
     (`review --mode=<id> --json`, `validate --mode=<id> --json`, and
     `status --view=<id> --json` where bindings admit it).
4. **English + zh-CN stay in lockstep.** Any PR that mutates this plan
   updates both files in the same batch, same rule as Wave-1 §8.6.
5. **Wave-3 trigger.** Within 7 days of Wave-2 PR-1 merge, produce a short
   metrics report covering: rendered reference byte ratios for the five
   Wave-2 skills; any warning-band body sizes; hydrate smoke results; and
   whether `variant-analysis` cross-wave references successfully link into
   `sast-orchestration` without duplication. Wave-3 scope is confirmed
   after that report is reviewed.

## 7. Out of scope

- Rewriting `skills_ref/` provenance; Wave-2 only adds pointers into it.
- Adding typed partials to any of the five skills. If a specific skill
  later needs a `CHECKLIST.tmpl` or `VERDICT.tmpl`, open a follow-up wave.
- Bundling any Wave-3 skill into Wave-2, even if its source happens to be
  ready. Wave-3 depends on Wave-2 metrics (see §6.5); mixing the two
  would defeat that gate.
- Any change to hydrate contract shape, selection paths, or
  infrastructure introduced in Wave-1. If a Wave-2 implementation PR
  would need such a change, stop and escalate instead of expanding scope.
