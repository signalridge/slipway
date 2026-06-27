# Structure

- `cmd/common.go`
  - Owns shared command helpers such as `resolveActiveChangeRef`,
    `resolveExplicitChange`, execution-summary loading, and invocation route
    decoration helpers used by public lifecycle commands.
- `cmd/status.go`
  - Owns the public `status` command route order: explicit `--change`,
    current-worktree binding, current-worktree archived fallback, then global
    active-change summary.
- `cmd/status_view_build.go`
  - Owns status JSON/prose projection, evidence pointers, freshness fields,
    gate display, and lifecycle timeline rendering.
- `cmd/next.go` and `cmd/next_context_build.go`
  - Own `next` command routing, read-only preview behavior, next-skill view
    construction, input context, and handoff context projection.
- `cmd/validate.go`
  - Owns validate JSON projection, artifact contract display, readiness
    projection, and route metadata for validation callers.
- `internal/state/store.go`
  - Owns active bundle discovery across visible workspace roots, strict
    `change.yaml` loading, worktree visibility checks, and persistence
    transaction ops.
- `internal/state/worktree_binding.go`
  - Owns git-local `worktree-binding.yaml` records used to resolve dedicated
    worktree authority without persisting absolute paths into tracked bundles.
- `internal/state/paths.go`
  - Owns canonical path derivation for runtime evidence, governed bundles,
    archive paths, and codebase map locations.
- `internal/state/verification.go`
  - Owns verification directory resolution, strict verification YAML loading,
    and resolved-change verification inventory helpers.
- `internal/state/lifecycle_event.go`
  - Owns lifecycle event path resolution, append/readback, full-log reads, and
    the new status-tail read surface planned by this change.
- `internal/engine/progression/readiness.go`
  - Owns governance readiness projection. It may accept preloaded verification
    records if needed, but `internal/state` must not import engine packages.
