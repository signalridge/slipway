# Architecture

Re-authored for change `resolve-github-issue-164-implement-transactional-multi-file`
(GitHub issue #164).

Question: where should transactional multi-file stage-transition writes live so
Slipway cannot leave partial governed bundle or evidence state when a later
write fails?

- Affected modules:
  - `internal/fsutil/atomic.go:14` exposes the current single-file durability
    primitive, `WriteFileAtomic`, using temp-in-dir, fsync, rename, and parent
    sync. The new work should compose with this primitive rather than weaken it.
  - `internal/engine/progression/advance_governed.go:337` through
    `internal/engine/progression/advance_governed.go:355` advances S1 plan
    substeps, scaffolds the governed bundle when entering `bundle`, and only
    then persists `change.yaml`.
  - `internal/engine/artifact/manager.go:241` through
    `internal/engine/artifact/manager.go:309` resolves required artifacts and
    writes missing scaffold-owned files in a loop with direct file writes.
  - `internal/state/store.go:511` through `internal/state/store.go:550`
    persists `change.yaml` and then records the machine-local worktree binding.
  - `internal/engine/progression/advance_governed.go:403` through
    `internal/engine/progression/advance_governed.go:424` materializes
    `wave-plan.yaml` before the S1-to-S2 transition state is saved.
  - `internal/state/wave_execution.go:72` through
    `internal/state/wave_execution.go:90` writes `wave-plan.yaml`; lines 133
    through 181 build and save that plan from `tasks.md`.
  - `internal/engine/progression/stale_evidence_recovery.go:137` through
    `internal/engine/progression/stale_evidence_recovery.go:182` removes stale
    skill verification, wave plan, and execution-summary files before
    `internal/engine/progression/stale_evidence_recovery.go:238` saves the
    reopened lifecycle state.
  - `internal/engine/progression/advance_governed.go:521` through
    `internal/engine/progression/advance_governed.go:541` owns evidence removal
    helpers that also prune digest records.
- Dependency flow:
  - S1 bundle progression: `AdvanceGoverned` changes the plan substep,
    scaffolds non-deferred governed artifacts, then saves `change.yaml`.
  - S1-to-S2 progression: `AdvanceGoverned` materializes a wave plan, changes
    lifecycle state, then saves `change.yaml`.
  - Stale reopen: `reopenToStaleStage` removes evidence files and digest
    entries, mutates change state, then saves `change.yaml`.
  - Single-file persistence already routes through `fsutil.WriteFileAtomic`;
    issue #164 needs an all-or-nothing boundary around ordered sets of those
    file mutations.
- Architectural boundary:
  - Add a focused file transaction helper in `internal/fsutil`.
  - Adapt transition call sites to express file writes/removes inside a
    transaction boundary.
  - Keep directory archive and bundle relocation outside this change unless
    tests prove the same file-set failure class; archive already has directory
    rollback coverage.
- Blast radius:
  - `internal/fsutil` for transaction mechanics and rollback diagnostics.
  - `internal/engine/progression` for transition wrapping.
  - `internal/engine/artifact` for scaffold-owned artifact write integration.
  - `internal/state` only where wave-plan materialization needs a transaction
    seam or operation builder.
  - Targeted tests in the same packages.
