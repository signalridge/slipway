# Stack

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- Language: Go.
- CLI framework: Cobra command tree under `cmd/`.
- Persistence: YAML governed bundle records under `artifacts/changes/<slug>/`
  and runtime task evidence under `.git/slipway/runtime/changes/<slug>/`.
- Template/content layer: generated command prompt bodies from
  `internal/tmpl/templates/_partials` and command metadata from
  `internal/toolgen`.
- Test framework: Go `testing` plus `testify` assertions.
- Verification commands expected for this change: focused Go package tests,
  `go test ./cmd -count=1`, `go test ./internal/toolgen -count=1`,
  `go test ./... -count=1`, `git diff --check`, and Slipway governance
  validation.
