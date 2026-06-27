# Architecture

- Question: Which state-read hot paths must be optimized so `status`, `next`,
  and `validate` remain interactive across many worktrees, bundles, and
  verification records without weakening current-worktree authority?
- Entry points: `cmd/status.go` routes explicit `--change` through
  `loadStatusChangeBySlug`, worktree-local status through
  `statusChangeFromCurrentWorktreeBinding`, then root/multi-active status
  through `state.ListChanges` and `resolveStatusRouteForRoot`
  (`cmd/status.go:226`, `cmd/status.go:252`, `cmd/status.go:285`,
  `cmd/status.go:365`). `cmd/next.go` and `cmd/validate.go` both call
  `resolveActiveChangeRef` before building their JSON views
  (`cmd/next.go:341`, `cmd/validate.go:173`).
- State authority: `internal/state` owns bundle discovery, strict
  `change.yaml` loading, worktree visibility, local runtime binding, and
  lifecycle JSONL reads. `LoadChange` currently resolves candidate workspace
  roots for every slug load (`internal/state/store.go:437`), while
  `ListChanges` discovers slugs and then loads each slug again
  (`internal/state/store.go:780`).
- Current duplication: command views often reload a change after already
  resolving its slug. `status` reloads inside `showStatusForChange`
  (`cmd/status.go:514`), `next` reloads for route decoration after building
  its view (`cmd/next.go:379`), and `validate` reloads after
  `buildValidateViewForSlug` (`cmd/validate.go:189`).
- Read collaborators: `ResolveChangePaths` is a pure root/change path
  derivation (`internal/state/paths.go:29`); verification has both slug-based
  reads and resolved-change reads (`internal/state/verification.go:279`,
  `internal/state/verification.go:292`); lifecycle event reads currently decode
  the full log (`internal/state/lifecycle_event.go:123`,
  `internal/state/lifecycle_event.go:204`).
- Blast radius: keep changes inside command read assembly and `internal/state`
  read helpers. Do not change lifecycle append crash-safety, durable
  authority, mutation ordering, or `internal/state -> internal/engine`
  dependency direction.
