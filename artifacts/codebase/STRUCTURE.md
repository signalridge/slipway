# Structure

Re-authored for change `resolve-github-issue-164-implement-transactional-multi-file`
(GitHub issue #164).

- `internal/fsutil/`
  - `atomic.go`: existing atomic single-file write helper and temp artifact
    cleanup. Planned transaction work belongs here as a small sibling helper,
    likely `transaction.go` plus package tests.
- `internal/engine/artifact/`
  - `manager.go`: governed artifact schema resolution and scaffold-owned file
    materialization. The current scaffold loop creates missing non-deferred
    artifacts before the caller persists lifecycle state.
  - Planned tests should prove scaffold writes participate in a transaction
    when used from governed progression.
- `internal/engine/progression/`
  - `advance_governed.go`: primary mutating lifecycle transition function.
    It owns S1 plan substep progression, S1-to-S2 wave-plan materialization,
    and evidence/digest removal helpers.
  - `stale_evidence_recovery.go`: recovery reopen path that removes stale
    evidence and then saves reopened lifecycle state.
  - Planned tests should cover injected failure after at least one transition
    file mutation for both artifact creation and evidence removal.
- `internal/state/`
  - `store.go`: `SaveChange` persists the governed `change.yaml` authority
    with `fsutil.WriteFileAtomic`.
  - `wave_execution.go`: wave-plan generation and persistence from `tasks.md`.
    The implementation may need a transaction-aware save path or operation
    builder so `wave-plan.yaml` and the following state save are atomic as a
    file set.
  - `lifecycle_test.go`: existing archive rollback tests cover directory
    promotion, which is related evidence but not the issue #164 file-set class.
- `cmd/`
  - `preset_test.go`: contains a preset-specific scaffold rollback regression,
    useful as prior art but not sufficient for generic governed stage
    transition file-set rollback.
- `artifacts/changes/resolve-github-issue-164-implement-transactional-multi-file/`
  - Governed artifact bundle for this change. The plan targets code and tests;
    `assurance.md` remains deferred until review/verify stages.
