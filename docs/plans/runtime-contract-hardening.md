# Runtime Contract Hardening

> Status: Draft
> Author: @LuYixian
> Date: 2026-04-16

## Why This Needs A Separate Plan

`over-engineering-audit.md` identified a second cluster of issues that are not
"small cleanup". They cut across:

- intake substep routing
- governed planning progression
- worktree metadata binding
- intent-classification degradation reporting
- artifact reconciliation and amendment handling
- transition/history visibility in JSON surfaces

This is large enough that a one-shot patch would be hard to review, hard to
rollback, and likely to mix contract changes with persistence changes in the
same diff. The correct shape is a bounded multi-wave plan.

## Architecture Stance

Slipway should **not** move to "tool only reports raw facts, the AI caller
decides every substep". The better target, based on the current product shape
and external workflow systems, is:

- runtime-owned workflow progression
- explicit transition reasons
- explicit side effects
- no hidden persistence in checker/helper functions
- no silent degradation on JSON surfaces
- no read-only command mutating durable state

This is closer to:

- `superpowers`: workflow skills remain runtime-owned
- `gsd`: `next` advances the workflow automatically
- `spec-kitty`: runtime stays authoritative, but the state, waits, and failures
  are explicit

The goal is therefore **not** "remove automation". The goal is
**"keep automation, remove hidden decisions and hidden mutation."**

Because the JSON consumers live in the same repository (`SKILL.md` templates,
hook templates, command tests, and documented caller contracts), this plan does
**not** preserve incorrect field shapes for compatibility. When a JSON contract
is wrong, we replace it directly and update same-repo consumers in the same PR.

## Hard Rules

1. Runtime may choose the next substep or recovery action.
2. Every runtime-owned transition must surface a machine-readable reason and
   state/substep delta.
3. Read-only flows must not persist metadata, scaffold files, or thaw artifacts.
4. Checker/helper functions must be pure; persistence happens in explicit apply
   steps only.
5. JSON callers must receive structured authority surfaces; free-form text is
   human-only and non-authoritative.
6. JSON callers must be able to distinguish:
   - normal inference vs safe degrade
   - normal progression vs recovery-only progression
   - no-op evaluation vs artifact-creating side effect
   - stable artifact vs auto-amended artifact
7. Do not introduce a second progression kernel or compatibility shadow path.
8. Do not keep alias fields, dual-write shims, or parallel JSON contracts for
   the sake of same-repo callers.

## Confirmed Remaining Issues

| Item | Current Code Path | Decision | Target |
|------|-------------------|----------|--------|
| NEW-1 intake substep routing | `internal/engine/progression/advance_intake.go` | keep runtime-owned | encode route reasons/signals in structured `AdvanceSummary` |
| NEW-2 plan substep progression + scaffold | `internal/engine/progression/advance_governed.go` | keep runtime-owned | encode scaffold side effects in structured `AdvanceSummary`; verify read-only scaffold purity |
| NEW-3 worktree metadata binding in checker | `internal/engine/progression/validation.go` | remove hidden mutation | split pure derive/check from explicit apply/persist |
| NEW-4 plan recovery injection | `internal/engine/progression/advance_governed.go` | keep runtime-owned | encode recovery-only transitions in structured `AdvanceSummary` |
| NEW-5 silent intent degrade in JSON | `internal/engine/progression/inference.go`, `cmd/new.go` | keep safe degrade | add degrade fields directly to `createOutput` JSON |
| NEW-6 frozen artifact auto-amendment | `internal/engine/artifact/manager.go` | keep reconcile behavior | split reconcile projection facts from apply/persist semantics |
| NEW-7 silent clearing of substeps/history | `internal/engine/progression/advance_intake.go`, `advance_governed.go` | keep cleanup behavior | expose cleared substeps/fields in structured `AdvanceSummary` |

## Non-Goals

- Do not make the AI caller manually choose every intake or planning substep.
- Do not remove safe-degrade and replace it with optimistic failure handling.
- Do not add a new generic event platform unrelated to Slipway governance.
- Do not build a second "preview state machine" that diverges from the real one.
- Do not add a compatibility layer that keeps hidden mutation alive behind new
  JSON fields.

## Execution Waves

### Wave 1 — Pure Derivation And Degrade Transparency

**Outcome**

Remove the two highest-risk hidden behaviors first:

- checker-side mutation for worktree metadata
- silent safe-degrade on JSON intake surfaces

**Scope**

- `internal/engine/progression/validation.go`
- `internal/engine/progression/readiness.go`
- `internal/state/*` helpers used by governed worktree validation
- `internal/engine/progression/inference.go`
- `cmd/new.go`

**Changes**

1. Split the current worktree path/branch derivation into:
   - pure metadata extraction
   - pure worktree validation
   - explicit metadata apply/persist
2. Make the mutating caller responsible for the apply step instead of letting
   `GovernedWorktreeBlockers()` write back into `change`.
3. Adapt the readiness path to the same pure derivation contract:
   - readiness may derive blockers from a candidate copy
   - readiness must not persist scope metadata
   - readiness may discard derived metadata after blocker evaluation
4. Extend `slipway new --json` / `createOutput` with:
   - `intent_inference_degraded`
   - `intent_inference_degradation_reason`
5. Preserve conservative `SafeDegradeClassification()` behavior; only the
   observability contract changes.

**Evidence**

- unit tests proving worktree-check helpers do not mutate `change`
- unit tests proving readiness uses the pure worktree-derivation path without
  persisting metadata
- unit tests for degrade reason propagation on `new --json`
- regression test proving text mode still emits the degraded warning
- existing test suite passes: `go test ./... -count=1`

**Rollback Boundary**

One PR. If the new metadata/apply split is unstable, revert the split as a
single batch without touching later waves.

### Wave 2 — Structured AdvanceSummary Contract

**Outcome**

Keep runtime-owned routing, but hard-cut `AdvanceSummary` from a free-form
message carrier into the authoritative structured transition contract used by
`next` and any other advance-reporting surfaces.

**Scope**

- `internal/engine/progression/types.go`
- `internal/engine/progression/advance_intake.go`
- `internal/engine/progression/advance_governed.go`
- `internal/engine/progression/autopass.go`
- `cmd/next.go`
- any shared next-view / run-view output structs
- same-repo templates or tests that still assume `advanced.message`

**Changes**

1. Replace the current `AdvanceSummary` authority surface with structured
   fields such as:
   - `action`
   - `from_state`
   - `to_state`
   - `from_substep`
   - `to_substep`
   - `reason`
   - `recovery_only`
   - `signals`
   - `side_effects`
   - `cleared_fields`
   - `blockers`
2. Encode intake routing in the structured result instead of free-form prose:
   - `clarify -> research`
   - `research -> clarify`
   - `* -> confirm`
3. Encode planning recovery injection in the structured result instead of
   separate ad hoc JSON fields.
4. Encode scaffold side effects in the same structured result:
   - bundle scaffold occurred
   - created artifact IDs/paths
   - any other runtime-owned side effects from the advance attempt
5. Verify read-only scaffold purity. If preview/status/validate/projection-only
   review and next paths already avoid scaffold, lock that in with tests rather
   than adding new defensive logic.
6. Remove `AdvanceSummary.Message` from the authoritative JSON contract. If
   terminal rendering still wants prose, compute it from structured fields in
   the renderer or retain a human-only `message` that is explicitly
   non-authoritative.

**Evidence**

- focused tests for each intake routing branch
- focused tests for audit-failure recovery injection
- focused tests for scaffold side-effect reporting
- regression tests proving preview/status/validate/projection-only paths do not
  scaffold files
- JSON golden tests for `slipway next --json` and any other affected advance
  surface
- existing test suite passes: `go test ./... -count=1`

**Rollback Boundary**

One PR. Revert the summary contract as a single batch if needed; do not leave
parallel old/new transition schemas in the tree.

### Wave 3 — Reconcile Projection And Amendment Contract

**Outcome**

Keep reconcile behavior runtime-owned, but split read-only projection from
mutating apply semantics so amendment handling is explicit instead of hidden in
`ReconcileFromFilesystem`.

**Scope**

- `internal/engine/artifact/manager.go`
- `internal/engine/progression/readiness.go`
- `cmd/status_view_build.go`
- `cmd/validate.go`
- `cmd/review.go`
- `cmd/next.go`
- `cmd/done.go`

**Changes**

1. Split reconcile responsibilities into explicit projection/apply contracts:
   - read-only callers derive projected artifact state and amendment facts
   - mutating callers apply reconcile results to the live change
2. Audit reconcile callers in read-only paths:
   - if reconcile is needed, operate on a copy
   - otherwise introduce a projection-only reconcile variant
   - never pass the live persisted change into a read-only reconcile flow
3. Return amendment facts/events from reconcile logic instead of silently
   mutating state with no surfaced result.
4. Make mutating callers handle reconcile apply explicitly, including
   amendment-event surfacing where relevant.
5. Keep this wave scoped to artifact reconcile/amendment behavior only.
   Transition/substep cleanup already moved into Wave 2 via structured
   `AdvanceSummary`; do not introduce a separate durable transition log here.

**Evidence**

- tests proving read-only readiness/next/status/review projection does not
  persist artifact amendments
- tests proving mutating reconcile/apply paths surface amendment facts and
  persist the resulting state
- regression tests for any JSON surface that now exposes reconcile/amendment
  facts
- existing test suite passes: `go test ./... -count=1`

**Rollback Boundary**

One PR. If the projection/apply split proves unstable, revert it as a single
batch instead of keeping both reconcile models alive.

## Recommended Delivery Order

1. Wave 1
2. Wave 2
3. Wave 3

This order intentionally lands:

- pure-function cleanup before larger progression edits
- JSON contract transparency before reconcile projection/apply surgery
- transition-structure cleanup before artifact-amendment cleanup

## Estimated Effort

Total effort is **L**.

Practical shape:

- 3 reviewable PR waves
- each wave touches runtime + tests
- Waves 2-3 both change same-repo JSON contracts and therefore require
  template/test updates in the same PR

The main cost is not raw line count. The cost is preserving one authoritative
runtime while removing hidden state mutation and making side effects explicit.

## Exit Criteria

This plan is complete only when all of the following are true:

- JSON callers can detect degrade, recovery, route choice, scaffold, and
  amendment without reading plain text
- no checker/helper with a validation-style name persists durable state
- read-only flows are side-effect free
- mutating flows still own progression, but every automatic step is explicit
- latest transition details survive substep cleanup through the structured
  advance result
- no new compatibility kernel or shadow state machine was introduced
