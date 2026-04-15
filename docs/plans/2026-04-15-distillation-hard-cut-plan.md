# Distillation Hard-Cut Plan

**Status.** Proposed. If adopted, this plan removes `docs/distillation/` as a
live contract surface and replaces its machine-readable responsibilities with
code-local artifacts under `internal/engine/capability/` plus the existing
skill-local `SKILL.md` / `provenance.yaml` source tree.

This plan supersedes any active plan text that treats `docs/distillation/*` as
required live authority, including:

- `2026-04-11-skills-integration-plan*.md`
- `2026-04-11-skills-integration-plan-delivery*.md`
- `2026-04-14-skills-strengthening-plan*.md`

This plan does **not** change Slipway's progression kernel. `ResolveNextSkill`
remains the only progression authority.

Unless noted otherwise, every `*.md` doc / plan glob in this plan includes the
EN and zh-CN variants when both exist.

## 1. Problem

`docs/distillation/` currently mixes three different roles:

- machine-readable contract input
- human-readable design/reference prose
- rollout bookkeeping

That creates four concrete problems:

- `internal/engine/capability/by_source_test.go` parses
  `docs/distillation/by-source.md` as markdown table input, so a prose-oriented
  doc edit can silently mutate a CI gate.
- `docs/distillation/schema.md` and `docs/distillation/pr-checklist.md`
  describe live contract/gate behavior even though the actual enforcement lives
  in Go types, skill-local frontmatter, and tests.
- `docs/distillation/catalog.md` and `docs/distillation/routed-surfaces.md`
  duplicate registry/route truth that already exists in code and can drift.
- The repo already wants `docs/distillation/` deleted, so continuing to invest
  in it as a contract surface creates guaranteed churn and a second
  authority tree that we already know is temporary.

## 2. Goals

- Delete `docs/distillation/` entirely.
- Move the source-corpus reverse index into a machine-readable artifact owned by
  code.
- Keep per-skill source attribution in each skill's `provenance.yaml`.
- Re-anchor CI gates, comments, and active plan docs to code-local authorities.
- Avoid a new generated markdown mirror or any compatibility tail that keeps
  `docs/distillation/` half-alive.

## 3. Non-goals

- No new catalog skills.
- No change to progression, routing, or command semantics.
- No broad redesign of `provenance.yaml`; this plan only changes the repo-level
  reverse index and the docs surface around it.
- No second documentation tree that merely renames `docs/distillation/` to a
  different folder.
- No YAML-to-markdown regeneration pipeline for a doc surface we already intend
  to delete.

## 4. Final State

### 4.1 Authority map

| Old surface | Final authority | Notes |
|-------------|-----------------|-------|
| `docs/distillation/by-source*.md` | `internal/engine/capability/source_index.yaml` | machine-readable reverse index of the source corpus |
| `docs/distillation/catalog*.md` | `internal/engine/capability/registry*.go` + registry tests | no human-maintained mirror table remains |
| `docs/distillation/routed-surfaces*.md` | `internal/engine/capability/surfaces.go` + command consumers/tests | route-surface authority is established in code before this hard cut; this plan only removes the duplicate doc |
| `docs/distillation/schema*.md` | owning Go types, validators, and gate tests (`provenance.go`, `gates_test.go`, frontmatter compare tests, route tests) | contract lives with the enforcing code |
| `docs/distillation/domains/*.md` | skill-local `SKILL.md`, `provenance.yaml`, typed templates, and `references/` | no repo-wide reverse mirror remains |
| `docs/distillation/pr-checklist*.md` | CI gates plus acceptance checklists in active plan docs | no standalone distillation checklist doc remains |
| `docs/distillation/source-coverage-ledger-template*.md` | PR description artifact when needed, not a checked-in contract file | remove from repo |

The trailing coverage-snapshot table currently living in `by-source.md` is
historical bookkeeping only, not a live authority input. It is not migrated
into `source_index.yaml`; historical snapshots remain in git history and, when
needed, in plan / PR artifacts.

### 4.2 New source index contract

Introduce `internal/engine/capability/source_index.yaml` as the only
repo-level reverse index for the authoritative source corpus.

Initial shape:

```yaml
sources:
  - source: <vendor>/<source-skill>
    disposition: <standalone | posture-only | partial-only | view-only | route-only | absorbed | deferred>
    skills: [<catalog-skill-id>, ...]
    surfaces: [<non-catalog-surface-id>, ...]
    status: <B1 | B2 | B3 | B4 | B5 | B6 | shipped | n/a>
    notes: <optional human note>
```

Rules:

- `source` is unique across the file.
- `disposition` is an enum, not free text.
- `skills[]` names only registered catalog skill IDs.
- `surfaces[]` names only live, code-owned non-catalog landings such as
  `review-queue` or `observability-query`.
- Rows whose current landing is only a dead override or a not-yet-realized
  future surface stay in `notes`, not `surfaces[]`. Under today's code truth,
  that includes `trailofbits/second-opinion` and `openai/sentry` shaped rows.
- At least one of `skills[]`, `surfaces[]`, or `notes` must be non-empty so
  every row carries an intentional landing or explicit defer rationale.
- The file is sorted deterministically by `source`.
- `notes` is an optional short, single-line English note. Longer rationale or
  bilingual explanation belongs in plan docs or skill-local references, not in
  the machine-owned index.
- `notes` is not a prose-rescue sink. Distillation-only domain guidance and
  localized rationale are not migrated into `source_index.yaml`.

### 4.3 Provenance coverage semantics

`provenance-coverage-scan` changes its input source, not its policy:

- `standalone` and `partial-only` source-index entries must appear in at least
  one shipped catalog skill `provenance.yaml`.
- `posture-only`, `absorbed`, `view-only`, `route-only`, and `deferred` entries
  remain documented in `source_index.yaml` but are not provenance-gated.
- Every source named in any shipped catalog skill `provenance.yaml` must also
  appear in `source_index.yaml`.

This plan does **not** tighten the current gate from "presence" to
"exactly-one-owner". Ownership exclusivity can be revisited later, but is not
part of the hard cut.

### 4.4 Docs posture after the cut

After this plan lands:

- `docs/distillation/` does not exist.
- No code or tests read a markdown file under `docs/` to determine contract
  truth.
- No generated markdown mirror replaces the deleted directory.
- `source_index.yaml` stays English-only and machine-owned; it does not become
  a bilingual prose mirror.
- Distillation-only EN / zh-CN prose is not a preservation target in this hard
  cut. If a claim is not already represented by surviving code-local or
  skill-local authority, it is discarded with the deleted tree rather than
  migrated into a new appendix or reference surface.
- Historical rationale lives in plan docs; live contract lives next to the
  code/tests that enforce it.

### 4.5 Cross-plan ordering

Committed landing order:

- `2026-04-15-route-surface-refactor-plan*.md` PR-1 first
- then this plan's PR-1 / PR-2
- then `2026-04-15-route-surface-refactor-plan*.md` PR-2 / PR-3

No alternative interleaving is permitted on `main`. `surfaces.go` established
by the route-surface plan's PR-1 is the sole public-surface authority consumed
by this plan. No temporary `surfaces[]` bridge allowlist or second
hand-maintained surface table is permitted between the two plans.

## 5. Rollout

### PR-1 — Source Index Extraction

**Goal.** Replace markdown-driven reverse-index enforcement with a code-local
machine artifact.

#### Code scope

- New: `internal/engine/capability/source_index.yaml`
- New: `internal/engine/capability/source_index.go`
- New: `internal/engine/capability/source_index_test.go`
- Delete: `internal/engine/capability/by_source_test.go`
- Update: `internal/engine/capability/provenance.go` comments where they still
  describe `docs/distillation/by-source.md` as authoritative

#### Implementation

- Audit every current `by-source.md` row before freezing the schema, and record
  whether it maps to `skills[]`, `surfaces[]`, or `notes`.
- Any row whose current landing text is not a registered catalog skill and not
  a live code-owned non-catalog surface must be normalized into `notes`
  instead of forcing it into `surfaces[]`.
- Copy the current `by-source` corpus rows into `source_index.yaml` with a
  deterministic schema.
- Implement a loader/validator for `source_index.yaml` in the
  `capability` package.
- Validate `surfaces[]` directly against the surface-policy registry introduced
  by the route-surface plan's PR-1. `source_index` may consume that registry,
  but it must not introduce its own bridge allowlist or second surface table.
- Rewrite the provenance coverage gate to read `source_index.yaml` instead of
  parsing markdown with regex.
- Keep gate semantics unchanged apart from the input source swap described in
  §4.3.
- Freeze the legacy gated source set (`standalone` + `partial-only`) from
  `by-source.md` and assert that the same set remains provenance-gated after
  the migration. The hard cut may change the storage format, but must not
  silently shrink gate coverage.
- Do **not** add checked-in migration snapshots or migration allowlists. This
  hard cut trusts a direct corpus rewrite into `source_index.yaml`; protection
  comes from the gate-preservation and registry/provenance tests below, not
  from a second audit artifact tree.
- Delete markdown-table parsing logic from the gate implementation.
- Remove `by_source_test.go` entirely once the `source_index_test.go` coverage
  below has replaced both directions of the old gate.

#### Tests

- `TestSourceIndexValid`
- `TestSourceIndexCoverageMatchesProvenance`
- `TestProvenanceSourcesAppearInSourceIndex`
- `TestSourceIndexLegacyGatedSourceSetPreserved`
- `TestSourceIndexSkillsResolveInRegistry`
- `TestSourceIndexSurfacesUseKnownNonCatalogIDs`

#### Acceptance

- No test reads `docs/distillation/by-source.md`.
- The provenance coverage gate is driven only by `source_index.yaml` plus
  `provenance.yaml`.
- The gated source set (`standalone` + `partial-only`) is preserved across the
  storage-format swap.
- Reverse-index semantics remain stable across the swap.

### PR-2 — Distillation Surface Deletion

**Goal.** Delete `docs/distillation/` and re-anchor all live references to
code-local authorities.

#### Code / doc scope

- Delete: `docs/distillation/` and all EN / zh-CN variants under it
- Update: `internal/engine/capability/registry_b*.go` comments that cite
  `docs/distillation/catalog.md`
- Update:
  - `docs/plans/2026-04-11-skills-integration-plan*.md`
  - `docs/plans/2026-04-11-skills-integration-plan-delivery*.md`
  - `docs/plans/2026-04-14-skills-strengthening-plan*.md`
  - `docs/plans/2026-04-15-route-surface-refactor-plan*.md`

#### Implementation

- Remove the entire `docs/distillation/` directory in one cut; do not keep a
  compatibility subset.
- Do not add a prose-rescue batch, appendix migration, or skill-reference
  salvage pass for distillation-only EN / zh-CN text. This hard cut discards
  that tree unless the same content is already represented elsewhere by
  surviving authorities.
- Rewrite active plan text that currently names `docs/distillation/*` as live
  authority to instead point at:
  - `internal/engine/capability/source_index.yaml`
  - `internal/engine/capability/registry*.go`
  - `internal/engine/capability/{provenance,gates}_*.go`
  - `internal/tmpl/templates/skills/<skill>/SKILL.md`
  - `internal/tmpl/templates/skills/<skill>/provenance.yaml`
- Remove remaining `source-coverage-ledger-template*.md` references from
  `docs/plans/2026-04-14-skills-strengthening-plan*.md` in the same cut rather
  than staging that cleanup in a separate rescue PR.
- Remove code comments that cite `catalog.md` row numbers as if that doc were a
  stable authority.
- Do **not** create a new repo-wide markdown replacement for the deleted docs.

#### Tests

- `go test ./internal/engine/capability/... -count=1`
- `go test ./internal/toolgen/... -count=1`
- targeted `./cmd` tests for any command assertions touched while removing
  routed-surface doc references
- residue checks:
  - `rg -n "docs/distillation/" internal cmd docs/plans`
    returns zero hits outside this hard-cut plan family and historical plans
    explicitly predating this hard cut
  - `find docs -type d -name distillation` returns zero hits
  - `rg -n "by-source\\.md|catalog\\.md|routed-surfaces\\.md|schema\\.md|pr-checklist\\.md" internal`
    returns zero live-authority references

#### Acceptance

- `docs/distillation/` is absent from the repo.
- No live test or code path depends on a markdown doc under `docs/`.
- Active plan docs no longer present `docs/distillation/*` as frozen contract
  surface.
- No rescue-only appendix, migration ledger, or secondary reference tree was
  introduced to soften the delete.

## 6. Gates

Each PR in this plan runs:

- `go test ./internal/engine/capability/... -count=1`
- `git diff --check`

PR-2 additionally runs:

- `go test ./internal/toolgen/... -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- residue `rg` checks from §5 PR-2

## 7. Out of Scope

- Replacing `docs/distillation/` with another repo-wide documentation tree.
- Tightening provenance ownership from presence-based to exclusive ownership.
- Changing routed surface behavior as part of the docs hard cut.
- Reworking tool export shape or adapter-visible manifests.
