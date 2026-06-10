# Stack

- Language: Go.
- Template/content layer: markdown skill files and Go template rendering under
  `internal/tmpl`.
- Test framework: Go `testing` plus `testify` assertions already used in
  `internal/tmpl/templates_test.go`.
- Verification commands expected for this change: targeted `go test` for
  `internal/tmpl`, broader Go tests, `go test -count=1 ./...`, `go build ./...`,
  `go vet ./...`, `git diff --check`, and Slipway governance validation.
