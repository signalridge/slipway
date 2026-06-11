# Intent

## Summary
Resolve GitHub issue #164: implement transactional multi-file writes with rollback for Slipway stage transitions so simulated mid-transition failure leaves no partial bundle; reference local gsd-core src/phase.cts writePlanningFileSet during implementation.
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
irreversible_operations

## In Scope
- Add an all-or-nothing apply/rollback mechanism for Slipway governed multi-file lifecycle mutations where a stage transition writes more than one artifact or authority file.
- Apply that mechanism to stage-transition paths that can currently leave a governed bundle partially materialized or partially reopened if a later write fails.
- Preserve the existing single-file atomic write behavior; the new scope is the multi-file transaction boundary around those writes.
- Surface rollback failure as a fail-closed blocker/error that names the affected files to inspect, following the behavior pattern in local `gsd-core/src/phase.cts::writePlanningFileSet` without copying its TypeScript implementation.
- Add a regression test that simulates a mid-transition write failure and proves no partial governed bundle/artifact state remains.

## Out of Scope
- No broad redesign of the lifecycle state machine or artifact schema.
- No durable database-style journal, background recovery daemon, or cross-process transaction manager.
- No change to `slipway done` final archive semantics unless research proves the issue #164 failure class is in that path.
- No vendoring or direct port of GSD code.

## Constraints
- Use the current worktree's Slipway CLI and governed lifecycle as authority.
- Keep changes small and rooted in the issue #164 failure class: stage-transition multi-file artifact/authority writes.
- Preserve fail-closed behavior for irreversible-operations guardrails.
- Use existing Go repository style, `fsutil.WriteFileAtomic`, and current test helpers where possible.

## Acceptance Signals
- A test simulates a failure after at least one transition file write has been attempted and before the transition completes.
- After that simulated failure, pre-existing files are restored and newly-created transition files are absent; the governed change is not left with partial bundle materialization.
- Rollback failure handling names the file(s) that may require inspection and fails closed instead of reporting success.
- Targeted package tests covering the transactional path pass.
- Final governed readiness reaches `done-ready` through current-worktree `go run . validate --json` / governance health evidence.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which current Slipway stage-transition paths perform multi-file governed artifact/authority mutations and need the transactional wrapper for issue #164?

  Resolved from current code inspection plus local GSD reference:
  - Primary issue #164 surface: file-level governed transition mutations where a first artifact/evidence write or delete can succeed before a later `change.yaml`/verification write fails.
  - Cover S1 planning bundle materialization: `AdvanceGoverned` advances to `S1_PLAN/bundle`, `artifact.ScaffoldGovernedBundleForChange` may create artifact files, then `state.SaveChange` persists `change.yaml`.
  - Cover stale-evidence reopen: `reopenToStaleStage` removes verification/wave/execution summary files, mutates change state/evidence refs, then saves `change.yaml`.
  - Keep directory archive/relocation out of the first issue #164 wrapper unless tests prove they share the same file-set failure class; `ArchiveChange` already has explicit directory rollback, while `RelocateGovernedBundle` is a directory move path rather than the GSD-style multi-file write set.

## Deferred Ideas
- General-purpose filesystem journaling for arbitrary user edits.
- Automatic repair of every historical partially-mutated bundle shape.

## Approved Summary
- Confirmed by user on 2026-06-11 14:19:09 JST with response `确认推进`.
- Implement transactional, all-or-nothing file-set handling for governed stage-transition mutations that can currently leave a partial bundle when a later artifact, verification, or `change.yaml` write fails.
- First implementation scope covers S1 planning bundle materialization and stale-evidence reopen file mutations; directory archive/relocation semantics are excluded unless implementation tests prove they share the same issue #164 failure class.
- Rollback failures must fail closed and name the files requiring inspection, following the behavior of local `gsd-core/src/phase.cts::writePlanningFileSet` as a reference pattern without copying the implementation.
- Primary acceptance signal: a regression test simulates mid-transition failure after at least one file mutation and proves pre-existing files are restored, newly-created files are absent, and no partial governed bundle state remains.
