# Requirements
## Project Context
- Tech Stack: Go, Bash
- Conventions: small scoped fixes, fixture contracts in Go tests, shipped helper
  scripts under `internal/tmpl/templates/skills`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Shell

## Requirements

### Requirement: Separate `go list` stdout from stderr in `find-polluter-go.sh`
REQ-001: `find-polluter-go.sh` MUST build its package candidate list only from
`go list` stdout, not from stderr diagnostics or warnings.

#### Scenario: `go list` emits a warning while returning success
GIVEN `go list ./...` exits 0 and writes `go: warning: "./..." matched no packages` to stderr
WHEN `find-polluter-go.sh` enumerates packages
THEN the warning is not passed to `go test` as a package import path
AND the script does not report that it is checking one package.

### Requirement: Preserve distinct no-package and hard-failure diagnostics
REQ-002: `find-polluter-go.sh` MUST return a stable non-zero "no test packages
found" diagnostic when `go list` succeeds with no package stdout, and MUST keep
hard `go list` failures on the `go list failed for <glob>` path.

#### Scenario: no test packages match under an ancestor module
GIVEN a child directory below an ancestor `go.mod` contains no Go packages
WHEN `find-polluter-go.sh <pollution-path> ./...` runs from the child directory
THEN the script exits non-zero
AND output contains `no test packages found under ./...`
AND output does not contain `-- go test go: warning`.

#### Scenario: `go list` fails for a missing package tree
GIVEN a module root has no `./does-not-exist/...` package tree
WHEN `find-polluter-go.sh <pollution-path> ./does-not-exist/...` runs
THEN the script exits non-zero
AND output contains `go list failed for ./does-not-exist/...`
AND output does not collapse into `no test packages found`.

### Requirement: Cover issue #18 with deterministic fixture tests
REQ-003: `internal/toolgen/toolgen_test.go` MUST include fixture-contract
coverage for the ancestor-module/no-package case without relying on ambient
`$TMPDIR` ancestry.

#### Scenario: fixture constructs the issue #18 environment directly
GIVEN the test creates a temporary parent module and an empty child directory
WHEN it runs `find-polluter-go.sh` from the child directory
THEN the contract asserts the no-package diagnostic
AND asserts the `go list` warning was not parsed as a package.
