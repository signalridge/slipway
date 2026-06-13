# Testing

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

## Existing Coverage

- `cmd/progression_next_test.go:1727` through
  `cmd/progression_next_test.go:1774` proves `next` already projects
  done-ready for S4 with approved ship readiness.
- `cmd/cli_e2e_test.go:448` through `cmd/cli_e2e_test.go:480` proves `done`
  archives a ready change and `state.LoadArchivedChange` can load it afterward.
- `cmd/status_view_build_test.go` contains focused status-view tests and is the
  lowest-cost home for done-ready projection assertions.
- `internal/state/lifecycle_test.go` covers archived load path behavior, so this
  change should not need new state-layer archive tests unless archive discovery
  changes.

## Gaps For Issues #195 And #196

- No status-view test asserts a top-level done-ready signal after S4 ship
  readiness passes.
- No command test asserts `status --json --change <slug>` after `done` returns
  an archived/done status view instead of `change_state_load_failed`.

## Verification Plan

- Add a focused status-view test that builds an S4 ship-ready change and asserts
  done-ready fields, finalization blocker, and narrative.
- Add a command-level test that finalizes a ready change, runs
  `status --json --change <slug>`, and asserts archived status metadata.
- Run focused tests for `./cmd`.
- Run `go test ./...`, `git diff --check`, and `go run . validate --json`.
