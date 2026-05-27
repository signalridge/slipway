# Decision

## Alternatives Considered

- **Ignore all governed archives.** Rejected because the user wants top-level
  governed records to be Git-managed and because OpenSpec demonstrates the
  value of durable archive records.
- **Track raw archive directories exactly as generated.** Rejected because raw
  records contain machine-local paths and proof bodies that should stay local.
- **Add a separate `slipway/changes/` publishing tree.** Deferred because it
  requires migration and duplicate lookup semantics beyond this fix.
- **Keep the existing layout but split top-level records from local proof
  subdirectories.** Selected because it fixes the immediate Git boundary while
  minimizing migration risk.

## Selected Approach

DEC-001: Keep active `change.yaml` as the complete runtime authority.

DEC-002: At archive time, sanitize the archived `change.yaml` snapshot:
- clear `worktree_path`
- keep `worktree_branch`
- rewrite artifact paths to archive-local relative filenames
- do not add schema compatibility shims for older archive variants

DEC-003: Add a managed, idempotent `.gitignore` block for Slipway local state:
- `/artifacts/codebase/`
- `/artifacts/changes/**/evidence/`
- `/artifacts/changes/**/events/`
- `/artifacts/changes/**/verification/`
- `/.worktrees/`

DEC-004: Ensure the ignore block from `init`, `new`, and `codebase-map`, because
those commands create or rely on local Slipway state.

DEC-005: Preserve local evidence/events/verification directories on disk after
archive; only their default Git visibility changes.

## Interfaces and Data Flow

- `internal/state` owns the reusable ignore block and archive snapshot
  sanitization helpers.
- `internal/bootstrap.InitWorkspace` ensures local-state ignore rules during
  workspace initialization.
- `cmd/new` ensures local-state ignore rules before scaffolding a governed
  change bundle and worktree.
- `cmd/codebase-map` ensures local-state ignore rules before writing
  `artifacts/codebase/`.
- `cmd/learn` tolerates absent lifecycle logs for archived records because
  archived `events/` is intentionally local-only.

## Rollout and Rollback

Roll out through normal CLI release. Active records keep the same runtime
authority model; newly written archive snapshots become the canonical Git-safe
record format. Older archive variants are not hidden behind compatibility
loaders in this change.

Rollback by reverting the touched code and docs. Existing archived records with
relative paths remain valid because relative artifact paths are accepted by the
current model and archive lookup is path-based.

## Risk

- Existing tests expect archived worktree paths and absolute artifact paths.
  Mitigation: update tests to assert portable records while preserving archive
  discovery.
- Ignoring `events/` reduces cross-clone learning signals. Mitigation: archived
  changes without local events are not treated as integrity failures, and
  durable summaries remain in top-level records.
- Broad ignore rules could hide user files. Mitigation: ignore only Slipway
  local-state paths and not all of `artifacts/`.
