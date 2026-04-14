# Distillation PR checklist (B0-B7 reference)

This checklist is retained as reviewer guidance from the B0-B7 phase.
Post-B8, CI gates enforce the same constraints automatically; use this list as
an explanatory fallback when reviewing failures.

## Schema (`schema-lint` equivalent)

- [ ] Frontmatter declares `skill_id`, `domain`, `function`, `tier`,
      `primary_attachment`, `summary`, `trigger_signals`, `evidence_contract`,
      `bindings`, `provenance_ref`.
- [ ] `skill_id` equals the directory name under `internal/tmpl/templates/skills/`.
- [ ] `summary` uses the `Use when ... / Triggers on ...` phrasing.
- [ ] `primary_attachment` and per-binding `attachment` are one of
      `posture` / `procedure` / `checklist` / `tool-recipe` / `report-schema`.
- [ ] `trigger_signals[]` use only the operators in §4 of `schema.md`.
- [ ] Every typed template named in `SKILL.md` body or binding exists on disk.

## Size (`size-lint` equivalent, tier-aware)

Warning-band overages are reviewer notes only. A blocking failure occurs only
when a skill body exceeds the tier hard-max without the required
`size_rationale`.

- [ ] T1: `SKILL.md` body ≤ 2 KB; warn 2-6 KB; rationale required above 6 KB.
- [ ] T2: ≤ 3 KB; warn 3-8 KB; rationale required above 8 KB.
- [ ] T3: ≤ 1.5 KB; warn 1.5-3 KB; `size_rationale` required above 3 KB.
- [ ] Any overflow example, counter-pattern, or narrative moved to
      `references/` or intentionally dropped. Inline tool-recipe differences
      may stay in `SKILL.md` when they fit the size budget.

## Binding (`binding-compare` equivalent)

- [ ] `bindings[]` in frontmatter mirrors the Go-owned capability registry
      entry for this skill exactly. Drift blocks the PR.

## Provenance (`provenance-coverage-scan` equivalent)

- [ ] `provenance.yaml` lists every upstream source the plan assigns to this
      catalog skill under `extracted`, `dropped`, or `conflicts_with`.
- [ ] No narrative source material left behind — either absorbed with a rule
      anchor, dropped with a reason, or moved to `references/`.

## Docs synchronization

- [ ] `docs/distillation/catalog.md` status column updated if the batch landed.
- [ ] `docs/distillation/by-source.md` status column updated for every source
      this PR touches.
- [ ] EN and zh-CN variants of every touched doc updated in the same PR.

## Kernel and boundary guardrails

- [ ] `ResolveNextSkill` unchanged.
- [ ] No new governed host added outside the existing 10.
- [ ] `--mode` / `--view` behavior preserves explicit-flag-over-auto precedence.
- [ ] CI gate automation (schema/size/binding/provenance) remains enforced in tests.
