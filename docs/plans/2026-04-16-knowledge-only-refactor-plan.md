# Knowledge-Only Refactor Plan

**Status.** Draft. Lands only after
`2026-04-15-skills-wave3-plan*.md` PR-1 / PR-2 / PR-3 and the Wave-3 closeout
report review complete. Once those prerequisites land, this becomes the sole
cleanup plan for removing `docs/distillation/`, provenance metadata, and the
remaining checked-in dead source.

## 1. Motivation

The project's final artifact is distilled knowledge — procedures, checklists,
iron laws, and evidence contracts — expressed in the author's own phrasing.
Upstream attribution metadata (`provenance.yaml`, `provenance_ref`,
`docs/distillation/`) was useful during the first distillation pass but is no
longer part of the desired end state. Keeping those artifacts checked in
creates two costs the project no longer wants to pay:

- **Ongoing maintenance.** Every new skill, reference, or script has to carry
  matching provenance rows, ledger entries, and frontmatter fields that add
  nothing to the shipped behavior.
- **Source coupling.** Source-named metadata keeps the distilled artifact
  psychologically tied to upstream repo names even though the bodies are
  independently authored. The user's stated goal is a knowledge-only
  project — "same function, different phrasing."

`skills_ref/` stays checked in as a curator workshop (reference material for
future re-imports). It is not a runtime dependency and is not exercised by any
test after this plan lands.

## 2. Non-goals

- No change to catalog skill bodies, trigger clauses, bindings, hydrate
  references, or evidence contracts. Knowledge surface is unchanged.
- No change to `skills_ref/` contents.
- No change to the route-surface / `surfaces.go` authority, the catalog
  registry contract, the hydrate contract, or tier budgets.
- No rename of existing skills or hosts.
- No new replacement metadata surface. The end state is deletion, not
  migration.

## 3. Final state

After this plan lands:

- No `provenance.yaml` exists under `internal/tmpl/templates/skills/` or
  `.codex/skills/slipway/`.
- `docs/distillation/` does not exist.
- No `SKILL.md` frontmatter under `internal/tmpl/templates/skills/` or
  `.codex/skills/slipway/` contains `provenance_ref`.
- No Go type carries a `ProvenanceRef` field; no constructor assigns one.
- `internal/engine/capability/by_source_test.go` does not exist.
- Provenance-loading / coverage helpers that only exist to enforce deleted
  metadata do not remain.
- `registry_b2.go`, `registry_b3.go`, `registry_b4.go`, and `registry_b5.go`
  contain no `docs/distillation/catalog.md` comments.
- Checked-in dead source for `differential-review` is gone from both the
  template tree and the checked-in generated mirror tree.
- No active plan doc (`docs/plans/*.md`) instructs future work to write or
  maintain provenance artifacts.
- `skills_ref/` remains checked in, unchanged, as curator reference material.
- No existing test or gate asserts the presence of deleted provenance
  artifacts.

## 4. Scope

### 4.1 Filesystem deletions

| Path | Action |
|------|--------|
| `internal/tmpl/templates/skills/*/provenance.yaml` | delete (25 files) |
| `.codex/skills/slipway/*/provenance.yaml` | delete (25 checked-in mirror files) |
| `docs/distillation/` | delete (recursive) |
| `internal/tmpl/templates/skills/differential-review/` | delete dead checked-in source |
| `.codex/skills/slipway/differential-review/` | delete dead checked-in mirror |

### 4.2 Go code changes

- Remove `Skill.ProvenanceRef` field from `internal/engine/capability/registry.go`.
- Remove every `ProvenanceRef: "provenance.yaml"` assignment from
  `registry_default.go`, `registry_b2.go`, `registry_b3.go`, `registry_b4.go`,
  and `registry_b5.go`.
- Delete `internal/engine/capability/by_source_test.go`.
- Remove provenance-loading / coverage helpers in
  `internal/engine/capability/provenance.go` and adjacent tests if they only
  exist to enforce deleted provenance artifacts. No stub replacements.
- Remove `// See docs/distillation/catalog.md …` comments from `registry_b2.go`,
  `registry_b3.go`, `registry_b4.go`, and `registry_b5.go`. Replace with a
  short batch comment (for example `// B3 security cluster.`) or nothing.

### 4.3 Template and checked-in mirror changes

- Remove `provenance_ref: provenance.yaml` from every `SKILL.md` frontmatter
  under `internal/tmpl/templates/skills/`.
- Delete the template-tree `provenance.yaml` files in the same PR; do not
  leave empty placeholders behind.
- Refresh the checked-in `.codex/skills/slipway/` mirror in the same PR so the
  repo-local generated `SKILL.md` files also drop `provenance_ref`, and delete
  the checked-in mirror `provenance.yaml` files in that same refresh. Do not
  leave the checked-in mirror in a stale pre-cleanup shape relative to the
  template tree.
- Delete the checked-in `differential-review` template / mirror directories in
  the same PR. Route-surface PR-3 already removed runtime authority; this plan
  removes the dead source.
- Where a surviving skill body still requires a deleted provenance deliverable
  (today `variant-analysis` says to "record the pattern in the provenance
  artifact"), make the minimum body edit required to remove that dead contract
  in the same PR.
- Do not broaden this PR into body rewrites beyond the minimal wording cleanup
  required by the metadata removal itself.

### 4.4 Test changes

- Delete the by-source markdown coverage gate
  (`internal/engine/capability/by_source_test.go`).
- Delete provenance-only tests / helpers under `internal/toolgen` and
  `internal/engine/capability` if they assert deleted artifacts rather than
  runtime behavior.
- Keep tests that cover hydrate, bindings, surfaces, size budgets, script
  contracts, and frontmatter-to-registry mirroring *after* stripping
  `provenance_ref` from their expected frontmatter shape.
- Refresh only the goldens touched by this cleanup PR. At minimum,
  `internal/toolgen/testdata/skill_tree_inventory.codex.golden` must be updated
  with `UPDATE_GOLDEN=1` after the filesystem deletions land; unrelated earlier
  route-surface golden churn is not owned by this plan.

### 4.5 Plan-doc changes

- `2026-04-15-route-surface-refactor-plan*.md`: remove transitional
  cleanup wording that becomes stale once this PR lands, and point the final
  deletion / cleanup step at this plan.
- `2026-04-15-skills-wave2-plan*.md`,
  `2026-04-15-skills-wave3-plan*.md`: strip sentences that require future work
  to write or update provenance artifacts. Wave-3 closeout must point here.
- All EN files and zh-CN mirrors stay in lockstep.

## 5. Execution

Single bundled PR after route-surface, Wave-2, and Wave-3 are complete:

1. **Code fields and comments.** Remove `Skill.ProvenanceRef`, its assignments,
   the by-source coverage gate, and dead provenance-only helpers.
2. **Templates and checked-in generated tree.** Strip `provenance_ref` from
   every `SKILL.md`, delete the `provenance.yaml` files, apply the minimal
   body-level cleanup required to remove any deleted `provenance artifact`
   contract, refresh the checked-in `.codex/skills/slipway/` mirror, and delete
   the checked-in `differential-review` dead source directories.
3. **Filesystem cleanup.** Delete `docs/distillation/`.
4. **Tests and goldens.** Remove provenance-only assertions, adjust
   frontmatter-shape fixtures, and refresh only the cleanup-affected goldens
   (at minimum the codex skill-tree golden).
5. **Plan docs.** Repoint the route-surface / Wave-2 / Wave-3 family to this
   cleanup PR and remove any now-stale cleanup wording that still assumes
   deleted metadata files remain as future work.
6. **Verify.** `go vet ./...`, `go test ./... -count=1`, and `init --tools
   codex --refresh` against a scratch directory to confirm generated trees no
   longer emit the deleted files.

## 6. Acceptance

- `rg -n "provenance.yaml" internal/tmpl/templates/skills .codex/skills/slipway`
  returns zero hits.
- `rg -n "provenance_ref" internal/tmpl/templates/ .codex/skills/slipway/`
  returns zero hits.
- `rg -n "provenance artifact" internal/tmpl/templates/skills .codex/skills/slipway`
  returns zero hits.
- `rg -n "docs/distillation" internal/ docs/plans/2026-04-15-route-surface-refactor-plan*.md docs/plans/2026-04-15-skills-wave*.md`
  returns zero hits.
- `rg -n "by-source.md|catalog.md" internal/engine/capability/` returns zero
  hits.
- `internal/tmpl/templates/skills/differential-review/` and
  `.codex/skills/slipway/differential-review/` do not exist.
- `go test ./... -count=1` passes.
- Generated codex skill tree contains no `provenance.yaml` under any skill
  directory.
- `skills_ref/` is untouched.

## 7. Interaction with existing plans

- **Route-surface / Wave-2 / Wave-3** — remain prerequisites. This plan does
  not reorder them; it is the cleanup PR that follows their closeout.
- **Wave-3 closeout** — once reviewed, this bundled PR becomes the only
  authorized cleanup step for `docs/distillation/`, provenance metadata, and
  the remaining `differential-review` dead source.
- **Task #10 golden refresh** — folds into step §5.4.

## 8. Out of scope

- Rewriting distilled bodies beyond the narrow wording cleanup required by
  metadata removal.
- Re-architecting the catalog registry, hydrate contract, or route surface.
- Touching `skills_ref/` contents.
- Introducing a replacement metadata format. If future re-import needs
  provenance again, that is a new plan, not this one.
