# Research

## Alternatives Considered

- **Ignore all `artifacts/changes/`.** Rejected. This would hide the top-level
  governed record files the user explicitly wants Git-managed and would diverge
  from OpenSpec's model, where change proposals, designs, tasks, and archived
  change records are durable repository artifacts.
- **Track raw archives exactly as written today.** Rejected. Existing archived
  `change.yaml` records can contain absolute `worktree_path` values and
  absolute artifact paths. Raw `events/`, `verification/`, and `evidence/`
  records also contain local runtime references that are useful on the
  originating machine but unstable in remote Git history.
- **Split project records from runtime proof by directory policy.** Selected.
  Keep top-level governed artifacts and a sanitized `change.yaml` eligible for
  Git, while ignoring `evidence/`, `events/`, `verification/`,
  `artifacts/codebase/`, and `.worktrees/`. This matches spec-kitty's tiered
  distinction between local evidence/runtime trails and durable project state.
- **Move Git-managed records to a new `slipway/changes/` tree.** Deferred. It
  would produce the cleanest long-term vocabulary but creates migration scope
  beyond the requested fix. The current change can make the existing layout
  precise and safe first.


## Unknowns
No unresolved planning unknowns remain. The requested direction is to avoid
backward-compatibility layers for older archive schema variants; this change
defines the canonical Git-safe format for newly written archived records.


## Assumptions
- `change.yaml` remains the active runtime authority. Archive-time sanitization
  must not remove active-state fields before `done` or `cancel`.
- `events/lifecycle.jsonl` is a trace surface in Slipway today, not the sole
  reducer authority for lifecycle state. This differs from spec-kitty's
  `status.events.jsonl` model and justifies ignoring Slipway events by default.
- Verification and evidence bodies remain locally useful after archive, but
  remote Git history should rely on `assurance.md`, `tasks.md`, and sanitized
  `change.yaml` summaries rather than raw proof directories.
- Precise ignore rules are safer than ignoring the whole `artifacts/` tree
  because future project-owned artifacts may legitimately be tracked.
- Existing older archives with obsolete fields may continue to surface as
  diagnostics until they are regenerated or migrated by an explicit future
  cleanup; this change does not hide them behind loader shims.


## Canonical References
- `artifacts/changes/archived/make-slipway-archived-change-records-git-safe-while-keeping-raw-evidence-events-verification-loc/intent.md` for the original request and intake context.
- `internal/state/lifecycle.go` for archive migration and archived
  `change.yaml` serialization.
- `internal/state/lifecycle_event.go` for lifecycle event trace semantics.
- `internal/state/store.go` and `internal/state/paths.go` for bundle,
  evidence, and codebase-map path ownership.
- `internal/bootstrap/init.go`, `cmd/new.go`, and `cmd/codebase_map.go` for
  entry points that create local Slipway state.
- OpenSpec local checkout: `docs/getting-started.md` lines describing durable
  `openspec/changes` and archive records.
- spec-kitty local checkout: `.gitignore`, `docs/trail-model.md`, and
  `docs/status-model.md` lines separating local evidence/runtime state from
  durable project state.
