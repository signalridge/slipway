# Testing

- Test layout: Go `*_test.go` files across `cmd/`, `internal/tmpl/`, and
  `internal/toolgen/`.
- Coverage hotspots: template content contracts are pinned in
  `internal/tmpl/templates_test.go`; generated adapter workflow contracts are
  pinned in `internal/toolgen/toolgen_test.go`; hook messages are pinned in
  `cmd/session_start_hook_test.go` and `cmd/context_pressure_hook_test.go`.
  Evidence: `internal/tmpl/templates_test.go:53-71`,
  `internal/tmpl/templates_test.go:586-612`,
  `internal/toolgen/toolgen_test.go:631-666`,
  `cmd/context_pressure_hook_test.go:68-117`.
- Coverage gaps: there is not yet a test that defines the `handoff.md`
  authoring contract or asserts it remains non-authoritative; this change
  should add those assertions.
- Verification commands: targeted tests should include `go test ./internal/tmpl/...`,
  `go test ./internal/toolgen/...`, and relevant `go test ./cmd/...` when hook
  text changes; final verification can include `go test ./...`.
- Fixture patterns: render templates with `Render(...)` for templated content
  and `Content(...)` for static template files. Evidence:
  `internal/tmpl/templates_test.go:56-70`,
  `internal/tmpl/templates_test.go:596-607`.
- Notes: add negative tests for governance bypass wording, because prior
  template regressions have come from positive-only assertions.
