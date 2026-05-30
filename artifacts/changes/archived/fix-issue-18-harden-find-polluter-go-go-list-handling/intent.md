# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go, Shell
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
fix issue #18: harden find-polluter-go go list handling
## Complexity Assessment
simple
Issue #18 identifies a bounded robustness defect in one shipped shell script and
its fixture-contract test. The blast radius is limited to tool generation
fixtures and the exported root-cause-tracing helper.

## In Scope
- Harden `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`
  so `go list` stderr warnings are not parsed as package import paths.
- Treat an empty package set from `go list ./...` as a stable "no test packages
  found" failure path even when an ancestor `go.mod` makes `go list` exit 0.
- Update `internal/toolgen/toolgen_test.go` fixture contracts to cover the
  ancestor-module/no-packages case described in issue #18.

## Out of Scope
- Changes to codebase-map / issue #17 behavior.
- Broad rewrites of root-cause-tracing skills or unrelated generated scripts.
- Removing or weakening existing successful polluter-detection coverage.

## Constraints
- Keep the exported script portable POSIX shell.
- Preserve the existing command-line interface:
  `find-polluter-go.sh <pollution-path> <package-glob> [-run <regex>]`.
- Keep diagnostics useful for both `go list` hard failures and empty package
  results.

## Acceptance Signals
- Targeted `go test ./internal/toolgen` passes.
- Full `go test ./...` passes.
- The updated fixture does not treat `go: warning: "./..." matched no packages`
  as a package name.

## Open Questions
None.

## Approved Summary
Confirmed from the user request to solve GitHub issue #18 on 2026-05-30T13:36:37Z:
fix the `find-polluter-go.sh` package discovery path so stderr warnings from
`go list` cannot become bogus package names, update the fixture contract for
the ancestor-module/no-package environment, and verify with targeted and full Go
tests. Excludes issue #17/codebase-map work and unrelated root-cause-tracing
changes.
