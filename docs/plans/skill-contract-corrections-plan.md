# Skill Contract Corrections Plan

Status: Draft
Date: 2026-04-19

## Goal

Fix three confirmed contract issues in the current skill system without
changing routed command surfaces, adding a new skill, or turning curated
`references/` shelves into filesystem-driven default hydrate sets.

This plan is a standalone repair batch. Do not fold it into:

- `docs/plans/coding-discipline-plan.md`
- any consolidation or deletion wave

## Hard Constraints

1. Keep the current routed/public command surfaces unchanged.
   In particular, do not change:
   - `review -> independent-review`
   - `repair -> root-cause-tracing`
   - `validate --focus spec-trace`

2. Do not add a new skill, host, focus alias, or manifest-only companion.

3. Keep default hydrate sets curated.
   Do not infer default hydrate membership from the mere presence of a file
   under `references/`.

4. Authoritative change points are:
   - `internal/engine/capability/registry*.go`
   - `internal/engine/capability/*_test.go`
   - `internal/tmpl/templates/skills/**`
   - generated skill output only as regeneration artifacts

## Batch 1: Goal-Verification Support Contract

Objective:
Eliminate the current silent truncation on the `goal-verification` host.

### Changes

1. Keep `fresh-verification-evidence` as a `goal-verification`
   host-embedded support.

2. Keep `coverage-analysis` as a `goal-verification` host-embedded support.
   It owns a verdict-shaped coverage gate and should stay host-visible.

3. Remove `goal-verification` host-embedded bindings from the recommendation
   boosters:
   - `property-testing`
   - `mutation-testing`
   - `performance-profiling`

4. Make those boosters discoverable through `command-auto -> validate`
   instead of host attachment:
   - add `command-auto -> validate` to `property-testing`
   - add `command-auto -> validate` to `mutation-testing`
   - keep `command-auto -> validate` on `performance-profiling`

5. Update authoring-side frontmatter to match the registry-owned binding truth
   for the affected skills.

6. Replace the generic resolver guard with exact `goal-verification` support
   expectations.
   Do not keep a test that merely asserts `len(Supports) <= 3`.

### Direct Touch Set

- `internal/engine/capability/registry_b4.go`
- `internal/engine/capability/resolver_test.go`
- `internal/tmpl/templates/skills/property-testing/SKILL.md`
- `internal/tmpl/templates/skills/mutation-testing/SKILL.md`
- `internal/tmpl/templates/skills/performance-profiling/SKILL.md`

### Acceptance

- `goal-verification` no longer relies on resolver truncation to hide matching
  supports
- the intentional host-visible support set for `goal-verification` is:
  - `coverage-analysis`
  - `fresh-verification-evidence`
- `property-testing`, `mutation-testing`, and `performance-profiling` remain
  available on `validate`
- no new routed/public surface is introduced

## Batch 2: Wave-Orchestration IRON LAW

Objective:
Make the primary execution host doctrinally consistent with the other governed
hosts.

### Changes

1. Add an `IRON LAW` block near the top of
   `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`.

2. Use this wording:

```text
IRON LAW: NO TASK EXECUTION WITHOUT A GOVERNED PLAN AND CONFLICT DETECTION
```

### Direct Touch Set

- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`

### Acceptance

- `wave-orchestration` renders an explicit `IRON LAW`
- no execution workflow semantics change beyond making the existing host gate
  explicit

## Batch 3: References Discoverability

Objective:
Fix the real discoverability gap without widening default hydrate scope by file
count.

### Changes

1. Treat `variant-analysis` as the immediate discoverability repair.
   Update the default-loaded `SKILL.md` so the local `references/` shelf is
   explicitly visible to the reader.

2. Keep `performance-profiling` curated.
   `profiling-recipes.md` is already discoverable from the default skill text.
   Do not auto-promote `distributed-tracing-playbook.md` into the default
   hydrate set only because it exists on disk.

3. Keep `property-testing` curated.
   The current five-file default hydrate set remains authoritative.
   `refactoring.md` and `reviewing.md` stay shelf-only unless a concrete route
   or host path proves they are required in the default hydrate contract.

4. Only add registry-owned `HydrateReferences` entries in this batch if a file
   is proven necessary on the default runtime path. Disk presence alone is not
   proof.

### Direct Touch Set

- `internal/tmpl/templates/skills/variant-analysis/SKILL.md`
- optional registry/frontmatter updates only if a curated hydrate addition is
  explicitly justified by runtime truth

### Acceptance

- `variant-analysis` reference material is discoverable from the default skill
  text
- no plan step assumes `references/*.md` automatically belongs in default
  hydrate
- `performance-profiling` and `property-testing` keep curated default hydrate
  scope

## Verification

### Required Verification

1. `go test ./internal/engine/capability/... -count=1`
2. `go test ./internal/toolgen/... -count=1`
3. `git diff --check`
4. regenerate skills via the normal Slipway generation path
5. run a scratch `slipway init --tools codex --refresh` check and inspect:
   - generated frontmatter for the affected catalog skills
   - generated `wave-orchestration/SKILL.md`
   - generated `variant-analysis/SKILL.md`

### Required Assertions

- the `goal-verification` support output is intentional, not cap-truncated
- registry and authoring-side frontmatter stay aligned for all touched skills
- `wave-orchestration` renders the new `IRON LAW`
- `variant-analysis` shelf discoverability is visible in default text
- no new routed/public surface appears
- no default hydrate set grows merely because extra files exist on disk

## Out Of Scope

- `coding-discipline`
- skill deletion or consolidation
- changing `collectSupports()` to return more than 3 entries
- adding new `validate --focus` aliases
- promoting all on-disk `references/` files into `HydrateReferences`
- changing `using-slipway-catalog.md` semantics
