# Structure

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

- `cmd/status.go`
  - Defines `statusView`.
  - Resolves explicit `--change` requests.
  - Routes active status to `showStatusForChange`.
  - Contains delete-recovery status diagnostics for broken active state.
- `cmd/status_view_build.go`
  - Builds governed status projections from readiness, execution context,
    timeline, blockers, and recovery.
  - Owns the status narrative.
- `cmd/status_render.go`
  - Renders status JSON through `statusJSONView` and text through
    `renderStatusText`.
  - Owns the first user-visible "what next" hint for text output.
- `cmd/common.go`
  - Provides shared active-change loaders and archived fallback behavior used by
    validate-like active commands.
- `cmd/status_view_build_test.go`
  - Focused unit tests for status view projection.
- `cmd/cli_e2e_test.go`
  - Command-level e2e tests around `done` and JSON command behavior.
- `internal/state/lifecycle.go` and `internal/state/store.go`
  - Archive load and archive path discovery helpers.
