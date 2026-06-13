# Integrations

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

- External integrations:
  - None. The change is local CLI/governance state projection behavior.
- Public CLI surfaces involved:
  - `slipway status --json`
  - `slipway status --json --change <slug>`
  - text `slipway status`
- Internal integration points:
  - `cmd/status.go` for route and view shape.
  - `cmd/status_view_build.go` for governed readiness projection.
  - `cmd/status_render.go` for text hints.
  - `internal/state.LoadArchivedChange` for terminal records.
