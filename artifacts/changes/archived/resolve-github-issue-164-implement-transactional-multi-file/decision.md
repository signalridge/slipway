# Decision

## Alternatives Considered

1. Add a focused `internal/fsutil` file transaction helper and route issue #164 transition file sets through it.
   This keeps the existing single-file `fsutil.WriteFileAtomic` durability primitive, adds one reusable all-or-nothing boundary for write/remove sets, and gives tests a deterministic failure seam. It matches the local GSD reference pattern of recording applied file writes and rolling them back in reverse order without porting the TypeScript implementation.

2. Add custom rollback code directly to each transition path.
   This would minimize new abstraction, but each path would need separate snapshot, restore, newly-created-file cleanup, and rollback-failure reporting. That increases the chance that future governed transitions drift from the irreversible-operations guardrail.

3. Build a durable journal or recovery daemon for every governed filesystem mutation.
   This would provide stronger crash recovery, but it is larger than issue #164 requires. The current issue is about synchronous multi-file stage-transition failure, not process-crash recovery or a database-style transaction log.

Selected direction: option 1.

## Selected Approach

Add a small file-oriented transaction primitive under `internal/fsutil` and adapt the identified governed transition surfaces to use it when a transition mutates more than one lifecycle artifact or authority file.

The transaction helper will snapshot existing files before mutation, track newly-created files, apply operations in order, and roll back in reverse order if a later operation fails. Write operations will continue to rely on the existing atomic single-file primitive where possible. Remove operations will snapshot file bytes before deletion so stale-evidence recovery can restore removed evidence when the reopened state save fails.

The first implementation scope is limited to the issue #164 file-set surfaces found in research:

- S1 planning bundle materialization before `change.yaml` persistence.
- Stale-evidence reopen removals before reopened `change.yaml` persistence.
- S1-to-S2 `wave-plan.yaml` materialization before `change.yaml` persistence.

Directory archive, bundle relocation, and broad lifecycle-state redesign remain out of scope because they are not the file-set write/delete failure class requested by the issue.

## Interfaces and Data Flow

New or changed interfaces:

- `internal/fsutil` will expose a file transaction API for ordered write and remove operations. Exact type names may change during implementation, but the API must carry operation path, file bytes for writes, file mode when relevant, and deterministic test failure injection.
- Artifact scaffolding will either build transaction write operations for missing scaffold-owned artifacts or accept a transaction-aware writer from progression code. The resulting behavior must not leave a partially scaffolded bundle if a later transition save fails.
- Progression code will wrap transition-specific file sets so artifact or evidence mutations and the following `change.yaml` save succeed or fail together.
- State and wave-plan persistence will continue to use existing state-store semantics, but wave-plan materialization before an S2 transition must participate in the same all-or-nothing boundary as the transition state save.

Data flow:

1. The transition code determines the concrete file mutations required for the lifecycle move.
2. It passes those mutations to the `fsutil` transaction helper.
3. The helper snapshots current file state, applies each operation in order, and records what has been applied.
4. On success, the transition reports advancement normally.
5. On operation failure, the helper rolls back applied operations in reverse order and returns an error.
6. On rollback failure, the helper returns an error that includes the original operation error and the affected file paths needing inspection.

## Rollout and Rollback

Rollout is internal to the Slipway CLI and is gated by the governed plan, wave execution, domain review, and final readiness checks. The implementation should land as a narrow code change with package-level regression tests before any broader workflow behavior is touched.

Verification commands:

- `go test ./internal/fsutil ./internal/engine/artifact ./internal/engine/progression ./internal/state`
- `go run . validate --json`
- Additional governed `go run . run --json --diagnostics` / evidence commands selected by the current lifecycle.

Rollback path:

- Revert the new `internal/fsutil` transaction helper and the transition call-site changes.
- Revert the new regression tests tied to those interfaces.
- Re-run the targeted package tests and `go run . validate --json` to confirm the governed bundle reflects the reverted scope.

If a runtime rollback failure is observed during execution, the command must fail closed. Operators should inspect the paths named in the error, rerun validation after manual repair if needed, and continue only through normal Slipway gates.

## Risk

- A naive helper could preserve only newly-created files and miss replacing original bytes. The helper must snapshot existing file contents before applying writes or removes.
- Rollback itself can fail. That path must report both the original failure and the rollback file path rather than hiding the partial state.
- Wrapping `change.yaml` persistence together with artifact operations may require careful call-site boundaries so machine-local worktree binding writes are not treated as governed artifact state.
- Scope creep into directory archive or bundle relocation would increase blast radius without directly addressing issue #164. Those paths stay excluded unless a test proves they share the same file-set failure class.
- The codebase map for this worktree was generated for an earlier issue, so direct source inspection and current-worktree tests are the authority for this plan.
