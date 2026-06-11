# Research

## Alternatives Considered

### Architecture
- Question: which governed stage-transition file mutations can leave partial state if a later write fails?
- Affected modules:
  - `internal/engine/progression/advance_governed.go:337` through `internal/engine/progression/advance_governed.go:355` advances the S1 planning substep, scaffolds bundle artifacts when entering `bundle`, then persists `change.yaml`.
  - `internal/engine/artifact/manager.go:283` through `internal/engine/artifact/manager.go:306` loops over required scaffold-owned artifacts and writes missing files with `os.WriteFile`.
  - `internal/state/store.go:511` through `internal/state/store.go:550` persists `change.yaml` through `fsutil.WriteFileAtomic` and then records the machine-local worktree binding.
  - `internal/engine/progression/stale_evidence_recovery.go:137` through `internal/engine/progression/stale_evidence_recovery.go:182` deletes stale verification, wave-plan, and execution-summary files before `internal/engine/progression/stale_evidence_recovery.go:238` saves the reopened `change.yaml`.
  - `internal/engine/progression/advance_governed.go:403` through `internal/engine/progression/advance_governed.go:424` materializes `wave-plan.yaml` before persisting the S2 transition state.
- Dependency chains:
  - S1 research -> S1 bundle: `AdvanceGoverned` -> `ensureGovernedBundleScaffolded` -> `artifact.ScaffoldGovernedBundleForChange` -> `state.SaveChange`.
  - Stale reopen: `AdvanceGoverned` -> `reopenToStaleStage` -> `clearRecoveryEvidence` / `removeVerificationRecordAndDigest` -> `state.SaveChange`.
  - S1 audit -> S2 execute: `AdvanceGoverned` -> `state.MaterializeWavePlan` -> `state.SaveChange`.
- Blast radius:
  - File mutation utilities under `internal/fsutil`.
  - Governed transition code in `internal/engine/progression`.
  - Artifact scaffolding in `internal/engine/artifact`.
  - Tests in `internal/engine/progression` and/or `internal/engine/artifact`.
- Constraints:
  - Preserve `fsutil.WriteFileAtomic` as the single-file durability primitive; it already uses temp-in-dir, sync, rename, and parent sync (`internal/fsutil/atomic.go:14` through `internal/fsutil/atomic.go:73`).
  - Preserve fail-closed irreversible-operations governance; no force-close or bypass path.
  - Do not move directory archive/relocation into this issue unless file-set tests prove the same failure class. `ArchiveChange` already has explicit directory rollback tests, while this issue targets multi-file stage-transition writes.

### Patterns
- Existing convention: single-file persistence routes through `fsutil.WriteFileAtomic` for `change.yaml` (`internal/state/store.go:542`), wave plans (`internal/state/wave_execution.go:89`), skill verification records, and execution summaries.
- Existing non-transactional gap: scaffold-owned artifacts are written by a loop with direct `os.WriteFile` (`internal/engine/artifact/manager.go:283` through `internal/engine/artifact/manager.go:306`), and the caller saves `change.yaml` only after the loop returns (`internal/engine/progression/advance_governed.go:353`).
- Existing stale reopen deletes files before saving reopened state: `clearRecoveryEvidence` removes verification files and prunes digests (`internal/engine/progression/advance_governed.go:521` through `internal/engine/progression/advance_governed.go:541`), while the state save happens later (`internal/engine/progression/stale_evidence_recovery.go:238`).
- GSD reference pattern: local `gsd-core/src/phase.cts:1302` records successfully applied writes, and on error reverses them in reverse order; if rollback itself fails, it names the affected file and warns that planning files may be inconsistent (`/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/phase.cts:1302` through `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/phase.cts:1323`).
- Codebase-map advisory: `artifacts/codebase/ARCHITECTURE.md:3`, `artifacts/codebase/TESTING.md:3`, and `artifacts/codebase/CONCERNS.md:3` show the populated map was authored for issue #156, so direct source citations are the planning authority for issue #164.

### Risks
- High: A failed post-scaffold `change.yaml` write can leave newly-created artifact files while the lifecycle state remains pre-transition, creating a partial bundle class issue #164 explicitly targets.
- High: A failed stale-reopen `change.yaml` write can remove verification evidence while the lifecycle state still points past the reopened stage.
- Medium: A transaction helper that snapshots files naively can accidentally preserve stale temp files or directory metadata; keep the helper file-oriented and explicit about create, write, and remove operations.
- Medium: Rollback can fail. The error must include the original failure plus file names that may need inspection, matching the GSD warning behavior while using Go error wrapping.
- Low: Directory archive/relocation could be confused with file-set transaction work. Keep it excluded unless a regression proves it belongs.
- Guardrail domains: irreversible_operations. The rollback path is itself a guardrail and must fail closed.
- Reversibility: The proposed helper restores prior file bytes or removes files that did not exist before the transaction; if rollback fails, the command returns an error naming the affected files and no success state is reported.

### Test Strategy
- Existing coverage:
  - `cmd/preset_test.go:315` covers preset scaffold rollback at command level but relies on preset-specific recovery, not a generic stage-transition file-set transaction.
  - `internal/state/lifecycle_test.go:398` and nearby tests cover archive rollback failure for directory promotion, which is a different failure class.
  - `internal/fsutil/atomic.go:14` through `internal/fsutil/atomic.go:73` covers the production primitive used by single-file writes, but not multi-file all-or-nothing semantics.
- Infrastructure needs:
  - Add a small test seam around the new transaction helper so tests can inject a failure after the first mutation without depending on chmod behavior.
  - Prefer package-level tests in `internal/fsutil` for helper semantics, then progression-level tests proving the helper wraps the issue #164 transition surfaces.
- Verification approach:
  - Unit test: write two files in one transaction, inject failure after the first write, assert the pre-existing file content is restored and the newly-created file is absent.
  - Unit test: remove and write in one transaction, inject failure, assert deleted files are restored.
  - Unit test: force rollback failure, assert the error includes the failed rollback file path.
  - Progression regression: simulate S1 bundle scaffold materialization failure after a scaffold file write and before `change.yaml` persistence, assert no partial scaffold file remains and lifecycle authority is unchanged.
  - Progression regression: simulate stale reopen failure after verification removal and before `change.yaml` persistence, assert verification files are restored and lifecycle authority is unchanged.

### Options
- Option 1: Add a focused `internal/fsutil` file transaction helper and update the issue #164 transition call sites.
  - Tradeoffs: Small reusable primitive, aligns with GSD's applied/reverse-rollback model, keeps single-file atomic writes intact, and gives deterministic test seams.
  - Cost: Requires adapting scaffold and stale-reopen paths to express file operations before applying them.
- Option 2: Add ad hoc rollback code directly in each transition path.
  - Tradeoffs: Lowest abstraction count for two call sites.
  - Cost: Duplicates rollback logic, makes rollback failure reporting inconsistent, and makes future multi-file transitions likely to miss the guardrail.
- Option 3: Add a durable journal/recovery layer for all governance mutations.
  - Tradeoffs: Strongest crash-recovery model.
  - Cost: Larger state-machine and repair-surface redesign, outside issue #164's requested GSD-style apply/rollback mechanism.
- Selected: Option 1. It is the smallest contract-correct solution that matches issue #164 and GSD's core mechanism while preserving Slipway's existing single-file atomic primitive.

## Unknowns
- Resolved: Which current transition paths need the transactional wrapper? -> S1 planning bundle materialization, stale-evidence reopen, and S1-to-S2 wave-plan materialization before `change.yaml` save are the file-level transition surfaces found in current source.
- Resolved: Should directory archive/relocation be in scope? -> Not for first implementation; current evidence points to file-set transition writes/deletes, while archive/relocation is directory movement with existing rollback/repair-forward semantics.
- Remaining: None.

## Assumptions
- A file-oriented transaction helper is sufficient for issue #164 because the acceptance text names multi-artifact stage transitions and the GSD reference is file-content write rollback, not directory promotion.
- Tests can rely on injected write/remove failure seams rather than platform-specific permission failures; this keeps rollback behavior deterministic across macOS/Linux/Windows.
- Scope may include S1-to-S2 `wave-plan.yaml` materialization because it writes a verification artifact before saving the transition state, the same file-set ordering risk as the named S1 bundle and stale reopen surfaces.

## Canonical References
- `https://github.com/signalridge/slipway/issues/164`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/phase.cts:1302`
- `internal/engine/progression/advance_governed.go:337`
- `internal/engine/progression/advance_governed.go:403`
- `internal/engine/progression/stale_evidence_recovery.go:137`
- `internal/engine/progression/stale_evidence_recovery.go:238`
- `internal/engine/artifact/manager.go:283`
- `internal/engine/artifact/manager.go:304`
- `internal/state/store.go:511`
- `internal/state/store.go:542`
- `internal/state/wave_execution.go:72`
- `internal/state/wave_execution.go:89`
- `internal/fsutil/atomic.go:14`
- `artifacts/codebase/ARCHITECTURE.md:3`
- `artifacts/codebase/TESTING.md:3`
- `artifacts/codebase/CONCERNS.md:3`
