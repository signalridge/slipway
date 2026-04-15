# Skills Strengthening Plan — Wave 2 (Draft)

**Status.** Draft. Depends on the full landing of
`2026-04-15-route-surface-refactor-plan*.md` PR-1 / PR-2 / PR-3 per that
plan's cross-ordering. Do not open implementation PRs for this wave until that
gate is met. Per-skill byte targets, reference inventories, and hydrate
binding rows below are provisional and must be re-validated against the latest
Wave-1 metrics baseline and the post-refactor surface model before the first
Wave-2 PR lands.
Wave-1 already pushed `gha-security-review` into the high-warning band near the
64 KiB reference cap. Any later wave that reopens that skill must budget
collapse/defer work before adding material and must recompute the live byte
total instead of trusting an older snapshot.

## 1. Motivation

Wave-1 shipped the shared plumbing: PR-0 support-file export for non-catalog
skills, PR-4a/4b hydrate wiring across three selection paths, and a T1/T2
target budget lift. Wave-2 is defined against the post-route-surface target
state: raw `--mode=<skill-id>` / `--view=<skill-id>` have been removed from
the public surface, `differential-review` has already been absorbed into
`independent-review`, and `BindingCommandManual` no longer acts as a public
mechanism. This wave is the first pattern-application pass on top of that
refactored surface. It applies the same treatment (references + selective
script lift + hydrate wiring) to four catalog skills whose upstream sources
share a parallel `references/` or `resources/` shelf:

- Three Trail of Bits analysis skills (`variant-analysis`,
  `property-testing`, `mutation-testing`). Each has a well-bounded upstream
  reference shelf, so reference templates and fixture contracts are
  naturally reusable across them.
- `performance-profiling`, whose upstream
  (`alirezarezvani/performance-profiler` + `wshobson/distributed-tracing`)
  contributes both reference-worthy source material and one liftable helper,
  but that helper only clears the bar if it is renamed and documented honestly
  as a repo-level performance scan. Wave-2 does **not** treat it as a process /
  binary profiling launcher.

`differential-review` is **not** part of this wave: the route-surface
refactor's PR-3 absorbs its diff-only obligations into `independent-review`
(with `TestIndependentReviewPreservesDiffOnlyRules` +
`TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract`
as the preservation contract) and removes the runtime registry entry. The
checked-in `differential-review` template / mirror directories are cleaned up
later in the final knowledge-only cleanup PR, but the skill is already gone
from the live routed surface before Wave-2 starts. Strengthening it here would
still be pure churn.

Post-refactor exposure for the four remaining Wave-2 skills:

- `property-testing` → `--focus property` on `validate`
- `mutation-testing` → `--focus mutation` on `validate`
- `variant-analysis` → suggested-only on `review` / `repair`; no public
  selector
- `performance-profiling` → suggested-only on `validate` / `status`; no
  public selector and no `--view` surface (the `--view=performance-profiling`
  row was removed by the refactor)

`coverage-analysis` is intentionally **not** part of Wave-2 or Wave-3. It
remains a shipped suggested-only verification booster, but the current template
is already within the T1 body target, ships no upstream `references/` /
`scripts` shelf worth a dedicated strengthening PR, and already attaches on the
post-refactor `goal-verification` host path. For this plan family, treat it as
an explicit no-op freeze rather than an omitted fifth verification skill.

This wave does not introduce new infrastructure. It is a scope-bounded
application of Wave-1 contracts to a single family.

## 2. Non-goals

- No change to `ResolveNextSkill` or `capability.Resolve()` decision
  semantics. The hydrate wiring added here reuses Wave-1 PR-4a fields.
- No new catalog skills in Wave-2; this wave strengthens existing skills only.
- No new typed partials (`PROSE.tmpl` / `CHECKLIST.tmpl` / `VERDICT.tmpl`)
  in Wave-2. The four skills in this wave are reference-heavy; their bodies
  are already within T2 target after Wave-1's budget lift. Typed partials are
  outside the current Wave-2 scope.
- No further tier-budget lift. T1 stays at 2.5 KB, T2 at 3.5 KB, T3 at
  1.5 KB. Skills that land in warning-band after this wave should resolve
  via `references/` rebalance, not another budget bump.
- No `skills_ref/` rewrite. Wave-2 only adds pointers into it.
- No process / binary profiling recipe runner in Wave-2. The only helper lift
  allowed here is the repo-level scan contract defined in §4.
- No Slipway-ruleset adapter helper in Wave-2. `find-variant.sh` may scaffold
  from upstream CodeQL / Semgrep templates, but it must not hard-bind to local
  `sast-orchestration` naming or pretend to emit a finished query.

## 3. PR-1 — References for four skills

**Goal.** Recover upstream reference shelves as condition-triggered
reference content, under the same distillation rubric as Wave-1 PR-1:
condition-triggered operational content stays in `references/`; narrative
and long examples are collapsed or dropped; source-aligned filenames where
practical.

### Planned references

| Skill | Planned `references/` | Source sections mapped | Curator synthesis (if any) | Notes |
|-------|------------------------|------------------------|----------------------------|-------|
| `variant-analysis` | `methodology.md`, `codeql-variant-queries.md`, `semgrep-variant-rules.md`, `variant-report-template.md` | `trailofbits/variant-analysis/METHODOLOGY.md`; `resources/codeql/*`; `resources/semgrep/*`; `resources/variant-report-template.md` | `codeql-variant-queries.md` and `semgrep-variant-rules.md` are per-tool digests of the `resources/codeql` and `resources/semgrep` subtrees; capture "multi-file digest" in the PR mapping log. | Overlaps with the checked-in `sast-orchestration/{codeql-*,semgrep-*}.md` refs already present in this repo. Cross-reference both, do not duplicate CodeQL or Semgrep foundations — point at the sast-orchestration refs for shared material. |
| `property-testing` | `design.md`, `generating.md`, `strategies.md`, `libraries.md`, `interpreting-failures.md`, `refactoring.md`, `reviewing.md` | `trailofbits/property-based-testing/references/*` (7 files, 1:1) | None | Source is already source-aligned; keep names verbatim. |
| `mutation-testing` | `optimization-strategies.md`, `configuration.md` | `trailofbits/mutation-testing/references/optimization-strategies.md`; `trailofbits/mutation-testing/workflows/configuration.md` | None | Workflow content collapsed into a sibling reference; log the flat-map in the PR mapping log. |
| `performance-profiling` | `profiling-recipes.md`, `distributed-tracing-playbook.md` | `alirezarezvani/performance-profiler/references/profiling-recipes.md`; curated synthesis from `wshobson/distributed-tracing/SKILL.md` | `distributed-tracing-playbook.md` is curator-authored (no upstream reference file); record the reason in the PR mapping log: upstream ships only SKILL.md and its tracing material does not belong in the body. | Keeps the body focused on profiling workflow while tracing material becomes on-demand. |

### Cross-wave overlap handling

- `variant-analysis` vs `sast-orchestration`: both reference CodeQL and
  Semgrep content. Wave-2 variant-analysis references must link into the
  checked-in `sast-orchestration/codeql-*.md` and
  `sast-orchestration/semgrep-*.md` reference files already present in this
  repo for foundational content, and only restate what is specific to variant
  discovery / ruleset evolution. No line-for-line duplication. PR notes must
  list which sast-orchestration references are linked.
- If linking across skills would require a runtime feature (e.g.,
  cross-skill hydrate references), stop and escalate — Wave-2 does not
  introduce such a feature. Manual prose cross-reference inside the
  reference body is the intended mechanism.
- Wave-2 does **not** own any provenance bookkeeping. The committed scope is to
  strengthen the four skills; any metadata or source-coverage cleanup belongs
  only to `2026-04-16-knowledge-only-refactor-plan*.md`, which is the single
  authorized removal step.

### Code changes

- `internal/tmpl/templates/skills/<id>/references/*.md` — new files per
  the table above.
- `internal/tmpl/templates/skills/property-testing/SKILL.md` and
  `internal/tmpl/templates/skills/mutation-testing/SKILL.md` — add
  `hydrate_references:` frontmatter in the typed record shape introduced by
  Wave-1 PR-1 (`name`, `reason`). Do not reshape the frontmatter contract.
- `internal/engine/capability/registry_b4.go` — populate
  `Skill.HydrateReferences` only for `property-testing` and
  `mutation-testing`. Pattern mirrors Wave-1 PR-4a. `variant-analysis` and
  `performance-profiling` still land references on disk in Wave-2, but do
  **not** get dormant registry/frontmatter hydrate metadata before a concrete
  routed consumer exists.

### Tests to add / extend

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` — extend
  input list to include the four Wave-2 skill IDs (`variant-analysis`,
  `property-testing`, `mutation-testing`, `performance-profiling`).
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles` —
  extends automatically by virtue of registry growth; assert the two new
  hydrate-bearing skills resolve.
- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  (added in Wave-1 PR-4a) — extends automatically for
  `property-testing` / `mutation-testing`.
- `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget` — caps
  remain 24 KB / file, 64 KB / skill total.

### Acceptance

- Each reference file ≤ 24 KB; total references per skill ≤ 64 KB.
- Rendered-tree diff shows the new `references/` directories and the expected
  hydrate/frontmatter additions for the affected skills.
- PR notes include the per-skill source-depth byte-ratio table
  (`rendered_reference_bytes / selected_source_bytes`) against named source
  sections, plus a "mapped / collapsed / deferred" log.
- No reference file reproduces ≥ 50 % of the owning `SKILL.md` body
  line-for-line (manual review rule, same as Wave-1).
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  passes.

## 4. PR-2 — Honest helper lifts

**Goal.** Ship two honest helper lifts whose contracts stay grounded in the
upstream sources: one renamed repo-level performance scan for
`performance-profiling`, and one template scaffold generator for
`variant-analysis`.

| Script | Owning skill | Purpose | Lift source |
|--------|--------------|---------|-------------|
| `scripts/repo-performance-scan.py` | `performance-profiling` | Narrowed and renamed lift of `alirezarezvani/performance-profiler/scripts/performance_profiler.py`. Input: project directory path. Output: deterministic text / JSON report over large files, dependency counts, and bundle/build indicators. | `alirezarezvani/performance-profiler/scripts/performance_profiler.py` |
| `scripts/find-variant.sh` | `variant-analysis` | Template scaffold helper over the upstream `resources/codeql/*` and `resources/semgrep/*` shelves. Input: engine + language + seed metadata. Output: a stable starter query / rule scaffold with the chosen upstream template body plus TODO placeholders; not a finished runnable query. | `trailofbits/variant-analysis/resources/codeql/*.ql`; `trailofbits/variant-analysis/resources/semgrep/*.yaml` |

### Constraints

- Reuse the Wave-1 PR-2 Python script contract: script passes
  `python3 -m py_compile`, shell helpers pass `bash -n`, missing runtimes fail
  fast, and no new export plumbing is introduced.
- Preserve the upstream helper's true contract surface. The lifted helper stays
  a repo scanner over a directory path; it must **not** grow target process,
  binary, profile-mode, flamegraph-launch, or load-test orchestration flags in
  this wave.
- PR notes must record both the lift source and the deliberate rename from
  `performance_profiler.py` to `repo-performance-scan.py`, plus any additional
  narrowing (for example output-shape cleanup).
- `find-variant.sh` must stay source-grounded to the upstream resource shelves.
  It may select and emit the corresponding CodeQL / Semgrep template for a
  requested engine/language pair, plus stable TODO placeholders for the seed
  location and abstraction notes, but it must **not** hard-code Slipway
  `sast-orchestration` ruleset names or claim to synthesize a finished query.

### Code changes

- `internal/tmpl/templates/skills/performance-profiling/scripts/repo-performance-scan.py`
  — add the renamed repo-scan helper.
- `internal/tmpl/templates/skills/performance-profiling/SKILL.md`
  — document the helper entrypoint as a repository scan, including accepted
  inputs, JSON/text output modes, and failure posture; do not describe it as a
  process profiler.
- `internal/tmpl/templates/skills/variant-analysis/scripts/find-variant.sh`
  — add the template scaffold helper.
- `internal/tmpl/templates/skills/variant-analysis/SKILL.md`
  — document the helper entrypoint as a starter scaffold generator over the
  upstream template shelves; do not describe it as a finished query generator.

### Tests to add

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` — extends.
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`:
  - `repo-performance-scan.py`: given a fixture project tree, assert stable
    report shape for both text and `--json` output.
  - invalid-path case: assert a stable actionable error when the input path is
    missing or not a directory.
  - `find-variant.sh`: given `--engine=codeql --language=python` and seed
    metadata, assert output contains the Python CodeQL template body plus stable
    TODO placeholders for the seed file/line and abstraction notes; given
    `--engine=semgrep --language=go`, assert the corresponding Semgrep scaffold.
  - `find-variant.sh` invalid engine/language case: assert stable usage or
    validation error output.

### Acceptance

- `repo-performance-scan.py` and `find-variant.sh` pass static checks and at
  least one fixture or failure-contract test each.
- `init --tools codex --refresh` writes both scripts into the generated skill
  tree.
- No Wave-2 text presents `repo-performance-scan.py` as a process / binary
  profiling launcher.
- No Wave-2 text presents `find-variant.sh` as a finished query generator or a
  Slipway-ruleset adapter.

## 5. PR-3 — Hydrate wiring on affected selection paths

**Goal.** Verify and, if needed, minimally fix hydrate behavior for the two
Wave-2 explicit-focus skills on their post-refactor public selection paths.
The two suggested-only skills land references on disk in PR-1, but do not get
routed hydrate metadata or output in this wave. No new infrastructure:
route-surface PR-2 already routes public aliases through surface policy, and
Wave-1 PR-4a / PR-4b already handle hydrate rendering once the underlying
records exist.

### First-wave binding table

Verify each row against the b4 registry constructors and the surface-policy
registry before authoring tests; the registries are authority.

| Skill | Post-refactor exposure | Selection path | Initial hydrated refs | First surfaced in |
|-------|------------------------|----------------|-----------------------|-------------------|
| `variant-analysis` | suggested-only (§5.2 of refactor plan) | resolver `SuggestedCapabilities[]` on `review` / `repair`; no public explicit selector | none in Wave-2; references are file-backed only | not wired in Wave-2 under the current surface model |
| `property-testing` | `--focus property` on `validate` (§5.3 of refactor plan) | explicit focus alias resolves through surface-policy to backing skill | `design.md`, `generating.md`, `strategies.md`, `libraries.md`, `interpreting-failures.md`, `refactoring.md`, `reviewing.md` | `validate --focus property` |
| `mutation-testing` | `--focus mutation` on `validate` (§5.3 of refactor plan) | explicit focus alias resolves through surface-policy to backing skill | `optimization-strategies.md`, `configuration.md` | `validate --focus mutation` |
| `performance-profiling` | suggested-only on `validate` / `status` (§5.2 of refactor plan); `--view=performance-profiling` removed (§5.5) | resolver `SuggestedCapabilities[]`; no public explicit selector, no `--view` surface | none in Wave-2; references are file-backed only | not wired in Wave-2 |

Note: the two suggested-only skills deliberately stop at file-backed
references in Wave-2. This plan does **not** add dormant
`Skill.HydrateReferences` / `hydrate_references:` metadata with no routed
consumer. Under the current surface model, suggested-only skills remain
file-backed only.

### Code changes

- No new `Skill.HydrateReferences` declarations in PR-3. The explicit-focus
  declarations for `property-testing` / `mutation-testing` already land in
  PR-1 together with the reference/frontmatter updates so the existing
  frontmatter-vs-registry gates stay consistent.
- Default expectation: no production-code changes in
  `internal/engine/capability/` or `cmd/` are needed here. If the post-refactor
  `--focus` path fails to surface the already-declared refs, PR-3 carries only
  the minimal resolver / command fix needed to restore the documented path.
- 32 KB hydrate output cap from Wave-1 PR-4b applies. `property-testing`
  with 7 refs is the closest to the cap; check total byte estimate in PR
  notes. If it exceeds 32 KB, split the skill into reference tiers (e.g.,
  keep `interpreting-failures.md` and `reviewing.md` out of the default
  hydrate set by listing only primary refs in `hydrate_references:` and
  letting the others remain file-backed on-demand reading).

### Tests to add

- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  — extends automatically.
- `internal/engine/capability/resolver_test.go` — add cases proving
  `property-testing` and `mutation-testing` resolve through the surface-policy
  `--focus` alias path to the expected hydrate slice, and that
  `variant-analysis` / `performance-profiling` do not surface hydrate keys or
  appear in `Supports` for unrelated invocations (refactor's
  `TestCommandScopedBindingsDoNotAutoPopulateSupports` already covers this
  direction; Wave-2 adds concrete signal fixtures).
- `cmd/hydrate_view_test.go` — golden cases:
  - `validate --focus property` lists `property-testing/design.md` (and the
    rest of the wired ref set within the 32 KB cap).
  - `validate --focus mutation` lists
    `mutation-testing/optimization-strategies.md`.
  - negative golden: `validate --focus property` does not list
    `variant-analysis/*` or `performance-profiling/*` hydrate keys, since
    those are suggested-only and not wired in Wave-2.

### Acceptance

- `property-testing` and `mutation-testing` surface hydrate keys on their
  `--focus` alias path.
- `variant-analysis` and `performance-profiling` ship references on disk but
  emit no routed hydrate keys on any public command surface under the
  post-refactor surface model.
- Per-skill selected hydrate body total ≤ 32 KB per invocation.
- No regression in existing `cmd/...` / `capability` golden tests.
- Attempting `validate --mode=property-testing` (or any other raw skill-id
  selector) fails with the refactor's `unknown_route_mode` usage error, not
  a silent fallback.

## 6. Execution order and gates

1. **PR-1 first.** References + required hydrate frontmatter + registry
   records. This is the one PR that actually changes authoring surface.
2. **PR-2 second.** Ship `repo-performance-scan.py` and the matching
   `SKILL.md` / script contract for `performance-profiling`, plus
   `find-variant.sh` and the matching `variant-analysis` contract updates.
3. **PR-3 third.** Hydrate wiring depends on the PR-1 references and
   frontmatter already being on disk.
4. Each PR runs the same three hard gates as Wave-1 §8.5, adapted to the
   post-refactor surface model:
   - `go test ./... -count=1`
   - `init --tools codex --refresh` on this repo, diff review on the
     resulting `.codex/skills/slipway/` tree.
   - Command smoke checks for the affected surfaces:
     `validate --focus property --json`,
     `validate --focus mutation --json`,
     `validate --list-focuses --format=json` (must include `property` and
     `mutation`),
     and a negative smoke asserting `validate --mode=property-testing` (or
     any other raw skill-id selector) returns the refactor's
     `unknown_route_mode` usage error.
5. **English + zh-CN stay in lockstep.** Any PR that mutates this plan
   updates both files in the same batch, same rule as Wave-1 §8.6.
6. **Wave-2 closeout gate.** Within 7 days of Wave-2 PR-3 merge, produce a
   short closeout / metrics report covering: rendered reference byte ratios
   for the four Wave-2 skills; any warning-band body sizes; hydrate smoke
   results for `--focus property` / `--focus mutation`; and whether
   `variant-analysis` cross-wave references successfully link into
   `sast-orchestration` without duplication. Wave-3 scope is confirmed only
   after this report is reviewed, and no Wave-3 implementation PR may open
   earlier.

## 7. Out of scope

- Rewriting `skills_ref/`; Wave-2 only adds pointers into it.
- Do not ship `performance-profiling/scripts/profiling-recipes.py` or any other
  process / binary profiling launcher contract in this wave. The upstream
  helper being lifted is a repo scanner and must stay described that way.
- Do not ship a Slipway-ruleset adapter variant of `find-variant.sh` in this
  wave. The helper must stay grounded in the upstream CodeQL / Semgrep
  templates rather than local ruleset naming.
- Adding typed partials to any of the four skills.
- Bundling any Wave-3 skill into Wave-2, even if its source happens to be
  ready. Wave-3 depends on the Wave-2 closeout gate (see §6 item 6); mixing
  the two would defeat that gate.
- Any change to hydrate contract shape, selection paths, or
  infrastructure introduced in Wave-1. If a Wave-2 implementation PR
  would need such a change, stop and escalate instead of expanding scope.
