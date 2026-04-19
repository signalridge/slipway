# Coding Discipline Plan

Status: Draft
Date: 2026-04-19

## Goal

Add `coding-discipline` as a new registry-owned catalog skill that attaches a
small, execution-facing discipline posture to the planning and execution hosts.

This plan introduces one additive skill without changing routed command
surfaces, replacing existing governance skills, or duplicating already-owned
verification doctrine.

The net-new doctrine is intentionally narrow:

- surface assumptions before silently choosing a path
- prefer the smallest implementation that solves the task
- make surgical changes and avoid drive-by edits

## Hard Constraints

1. This is a standalone plan. Do not fold this work into
   `docs/plans/skill-consolidation.md`.

2. Name the skill by capability, not by source attribution:
   - use `coding-discipline`
   - do not use `karpathy-guidelines`

3. Model the skill as a catalog skill, not as a technique-only emitted file.
   The implementation must participate in:
   - registry lookup
   - resolver-driven support attachments
   - `next --json` support hint output
   - generated catalog manifest emission

4. Keep `PrimaryAttachment = posture`.
   This skill is a behavioral constraint layer, not a new execution procedure.

5. Bind only to real hosts:
   - `plan-audit`
   - `wave-orchestration`

   Do not bind it to `plan-authoring`, because `plan-authoring` is a support
   skill attached to `plan-audit`, not a host.

6. Keep the doctrine delta-focused.
   Do not restate or absorb existing goal-driven execution / TDD / verification
   doctrine already owned by:
   - `plan-authoring`
   - `tdd-proof`
   - `goal-verification`

7. V1 must not add:
   - a new routed/public command surface
   - `procedure` attachments
   - `checklist` attachments
   - `BindingTechniqueHint`
   - deletion or replacement of existing skills

   Terminology guardrail:
   - `evidence_contract` describes the proof shape the skill owns
   - `attachment` describes what may bind to a host or export surface
   - therefore `EvidenceContract: checklist` is allowed in V1 while
     `checklist` attachments remain forbidden

8. Host visibility is required.
   Resolver-visible bindings alone are not sufficient. The host prose for both
   `plan-audit` and `wave-orchestration` must explicitly name the
   `coding-discipline` overlay so the agent can see it without relying only on
   support-hint serialization.

## Baseline

Current code-truth facts that shape this plan:

- registry-owned catalog skills participate in resolver-driven support
  attachments, `next --json` support hints, hydration discovery, and generated
  manifest output
- technique-only emitted skills do not participate in those resolver paths
- `plan-audit` is the planning host for the plan stage
- `plan-authoring` is a support skill attached to `plan-audit`
- `wave-orchestration` is the execution host
- existing governance skills already cover:
  - bounded implementation planning
  - goal-driven execution
  - RED / GREEN / REFACTOR proof
  - post-execution verification and freshness checks

Implication:

- the new skill must be additive and posture-only
- the new skill must not duplicate existing procedure or verification doctrine
- the new skill must bind to hosts directly, not to support skills

## Decisions

1. Add `coding-discipline` as a new catalog skill.

Reason:
- technique-only emission would produce a file but would not enter the
  resolver-driven governance path
- the desired effect is runtime-visible support on governed planning and
  execution hosts, not passive documentation

Implementation note:
- author a new `internal/tmpl/templates/skills/coding-discipline/SKILL.md`
- register it in `DefaultRegistry()`
- emit it through the normal catalog render path

2. Keep the skill posture-only, host-attached, and catalog-exported.

Reason:
- the doctrine is about how the agent should behave while planning and
  executing, not about introducing a second workflow inside those hosts
- the `export-only -> using-slipway-catalog -> posture` binding keeps the
  skill listed in the generated catalog inventory so catalog readers can see
  that this posture exists without turning it into a routed host or ambient
  hint

Implementation note:
- use `PrimaryAttachment: posture`
- bind with:
  - `host-embedded -> plan-audit -> posture`
  - `host-embedded -> wave-orchestration -> posture`
  - `export-only -> using-slipway-catalog -> posture`

3. Keep `Domain: execution`, and explain why.

Reason:
- `Domain` is a coarse catalog classification label, not a host admission gate
- the available taxonomy does not provide a separate mixed
  planning-and-execution posture bucket
- the posture's sharpest effect is when the agent is choosing and applying code
  changes, even though one approved attachment point is `plan-audit`

Implementation note:
- keep `Domain: execution`
- document explicitly that `plan-audit` reach comes from bindings, not from the
  domain label

4. Keep the doctrine delta-focused and non-duplicative.

Reason:
- the original four-principle source contains one large section that overlaps
  strongly with existing Slipway skills
- copying that overlap into a new skill would add noise instead of net-new
  guidance

V1 doctrine:
- surface assumptions instead of silently inventing them
- choose the minimum implementation that satisfies the task
- make surgical changes and avoid opportunistic edits outside scope

V1 exclusions:
- do not restate `goal-driven execution`
- do not restate TDD proof requirements
- do not restate final verification freshness doctrine

5. Make host visibility explicit in prose, not only in bindings.

Reason:
- support bindings make the overlay discoverable to the resolver
- they do not by themselves guarantee that the active host text will make the
  relationship obvious to the agent

Implementation note:
- update `plan-audit` host prose to name the `coding-discipline` overlay
- update `wave-orchestration` host prose to name the `coding-discipline`
  overlay
- keep that host text short; do not inline a second full procedure

6. Keep the runtime impact additive.

Reason:
- this plan is not a consolidation or replacement pass
- existing skills and routed surfaces remain the source of truth for their
  current roles

Implementation note:
- do not remove or rename any existing skill in this plan
- do not change any current public route names
- expected drift is additive only:
  - one new registry-owned skill
  - one new manifest entry
  - one new host-attached support hint on each approved host

## Direct Touch Set

Authoring-side sources expected to change:

- `internal/tmpl/templates/skills/coding-discipline/SKILL.md`
- `internal/engine/capability/registry.go`
- `internal/tmpl/templates/skills/plan-audit/SKILL.md`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`

Likely verification and generation touch points:

- `cmd/next_skill_capability_hints_test.go`
- `internal/engine/capability/resolver_test.go`
- `internal/toolgen/testdata/skill_tree_inventory.codex.golden`
- generated `.codex/skills/slipway/coding-discipline/SKILL.md`
- generated host skill output only as regeneration artifacts
- generated `.codex/skills/slipway/using-slipway-catalog.md`

Treat this as the minimum expected touch set, not an exclusive list. If live
references or goldens fail, update them in the same batch.

## Execution Plan

Batching rule:
- Wave 1 and Wave 2 are one atomic delivery batch for implementation and
  review. Do not land, validate, or claim readiness for Wave 1 in isolation,
  because registry-visible bindings without host prose would leave the overlay
  only partially surfaced.

### Wave 1: Authoring And Registry Wiring

Objective:
Create the new skill in the authoring tree and wire it into the catalog path.

### Changes

1. Add `coding-discipline/SKILL.md` in the authoring tree.

2. Register `coding-discipline` in `DefaultRegistry()`.

3. Use catalog metadata that matches the intended runtime shape:
   - `ID: coding-discipline`
   - `Domain: execution`
   - `PrimaryAttachment: posture`
   - `EvidenceContract: checklist`

   Clarification:
   - `Domain: execution` is catalog classification, not host eligibility; host
     reach still comes from bindings, so the skill may attach to `plan-audit`
     and `wave-orchestration`

4. Add the approved bindings:
   - `host-embedded -> plan-audit -> posture`
   - `host-embedded -> wave-orchestration -> posture`
   - `export-only -> using-slipway-catalog -> posture`

5. Do not add `BindingTechniqueHint` in V1.
   Keep the skill host-scoped rather than broadening it into general ambient
   hinting.

### Acceptance

- `coding-discipline` exists in the authoring tree and registry
- the skill is emitted via the catalog render path
- registry/frontmatter comparison stays aligned
- Wave 2 host-prose updates are queued in the same delivery batch before merge
- no existing skill is renamed or removed in this wave

### Wave 2: Host Visibility And Doctrine Shaping

Objective:
Make the new overlay visible on the correct hosts while keeping the doctrine
small and non-overlapping.

### Changes

1. Update `plan-audit` host prose to explicitly name the
   `coding-discipline` posture overlay.

2. Update `wave-orchestration` host prose to explicitly name the
   `coding-discipline` posture overlay.

3. Keep the overlay wording short and execution-facing.

4. Preserve the V1 doctrine boundary:
   - assumptions must be surfaced
   - the minimum implementation should be preferred
   - changes must stay surgical

5. Keep the following out of the new skill:
   - bounded-task planning doctrine already owned by `plan-authoring`
   - RED / GREEN / REFACTOR proof already owned by `tdd-proof`
   - fresh completion verification already owned by `goal-verification`

### Acceptance

- both hosts name the overlay explicitly in their default text
- the new skill reads as posture, not as a second workflow
- no duplicated goal-driven / TDD / verification doctrine is introduced

### Wave 3: Verification, Regeneration, And Contract Checks

Objective:
Prove that the new skill is additive, host-visible, and contract-safe.

### Changes

1. Add or update tests that confirm:
   - `plan-audit` resolves `coding-discipline` as a support attachment
   - `wave-orchestration` resolves `coding-discipline` as a support attachment
   - generated `plan-audit` and `wave-orchestration` host text explicitly names
     the `coding-discipline` overlay
   - existing support attachments on those hosts do not regress

2. Regenerate skill output through the normal generation path.

3. Verify that catalog-manifest drift is additive only.

### Acceptance

- `next --json` support-hint output exposes `skill:coding-discipline` on the
  approved hosts
- `using-slipway-catalog.md` gains one additive `coding-discipline` entry
- generated host prose names `coding-discipline` on both approved hosts
- no routed public surface changes
- no existing support ID is renamed
- no existing catalog entry is removed

## Not In Scope

This plan does not:

- convert the idea into a technique-only emitted skill
- add a `karpathy-guidelines` alias
- bind the skill to `plan-authoring`
- add `procedure` / `checklist` attachments for `coding-discipline`
- collapse `coding-discipline` into an existing skill
- use this plan as a pretext to consolidate or delete other skills
- restate all four original source principles verbatim

## Verification

Every implementation batch for this plan must verify both code truth and
generated output.

### Required Verification

1. `go test ./... -count=1`
2. `go vet ./...`
3. `git diff --check`
4. regenerate skills via the normal Slipway generation path
5. run a scratch `slipway init --tools codex --refresh` check and inspect:
   - generated `coding-discipline/SKILL.md`
   - generated `plan-audit` and `wave-orchestration` host text
   - generated `using-slipway-catalog.md`
   - representative `next --json` support hints on the approved hosts

### Required Assertions

- `coding-discipline` is present in the registry and generated catalog output
- `coding-discipline` is absent from unrelated hosts
- `coding-discipline` appears on `plan-audit` and `wave-orchestration`
- host prose explicitly names the overlay on both hosts
- the new skill does not duplicate doctrine already owned by
  `plan-authoring`, `tdd-proof`, or `goal-verification`
- resolver-visible drift is additive only and limited to the approved new skill

## Final Outcome

Target end state after this plan lands:

- one new registry-owned catalog skill: `coding-discipline`
- one additive manifest entry for `coding-discipline`
- one additive host-attached posture overlay on:
  - `plan-audit`
  - `wave-orchestration`
- no new routed/public command surfaces
- no replacement of existing governance skills
- no duplication of existing goal-driven / TDD / verification doctrine
