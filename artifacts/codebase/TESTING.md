# Testing

- Test layout: Go *_test.go files
- Coverage hotspots:
  - `internal/tmpl/templates_test.go` covers generated template contracts,
    stale text removal, prompt surfaces, and skill export expectations.
  - `internal/tmpl/wave_isolation_content_test.go` covers wave dispatch
    isolation phrasing.
  - `internal/toolgen/toolgen_test.go` covers exported host skill inventory,
    generated directory layouts, and tool-specific surface behavior.
- Coverage gaps:
  - Existing tests assert some wave dispatch language, but do not yet guard that
    goal-verification and worktree-preflight stay thin-host/summary-first.
  - Generated skill contract tests should prevent future regressions that move
    long command output or source-file reading back into main host context.
- Verification commands: go build ./...; go test ./...
- Fixture patterns:
  - Template tests commonly read from the embedded template FS and assert on
    rendered markdown substrings.
  - Toolgen tests build generated trees in temporary directories and inspect
    emitted skill/prompt files.
- Notes:
  - Source references: `internal/tmpl/templates_test.go`,
    `internal/tmpl/wave_isolation_content_test.go`,
    `internal/toolgen/toolgen_test.go`.
