# Decision
## Project Context
- Tech Stack: Go, Bash
- Conventions: keep helper scripts deterministic; fixture contracts exercise
  shipped script behavior through `runCommandInDir`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Shell

## Alternatives Considered

1. Test-only tolerance for either `go list failed` or `matched no packages`.
   - Pros: smallest test change.
   - Cons: leaves the shipped helper parsing stderr warnings as package names.
2. Force the test temp directory outside every module.
   - Pros: preserves the old assertion.
   - Cons: misses the issue #18 developer-machine failure mode and is brittle
     across environments.
3. Fix script package discovery and add deterministic fixture coverage.
   - Pros: fixes the real helper robustness defect, keeps hard failures distinct,
     and covers the ancestor-module/no-package case directly.
   - Cons: small increase in script complexity to manage a temporary stderr file.

## Selected Approach
Use approach 3. Capture `go list` stdout into the package list and stderr into a
temporary file. Re-emit stderr for operator visibility, but only parse stdout
into `PKGS`. Keep empty stdout on the existing `no test packages found` branch,
and keep non-zero `go list` exits on the `go list failed for <glob>` branch.

This direction honors the documented constraints:
- Keep the exported script's CLI unchanged:
  `find-polluter-go.sh <pollution-path> <package-glob> [-run <regex>]`.
- Preserve useful diagnostics for both hard `go list` failures and empty package
  results.
- Stay inside the existing Bash implementation style.

## Interfaces and Data Flow
No public interface changes.

Internal data flow changes:
- Before: `go list` stdout and stderr were merged into `LIST_OUTPUT`, then
  non-empty lines were sorted into `PKGS`.
- After: `go list` stdout remains `LIST_OUTPUT`; stderr is captured separately,
  re-emitted, and never sorted into `PKGS`.

## Rollout and Rollback
Rollout is a normal source change to the script template and fixture tests.
Rollback is a file-level revert of:
- `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`
- `internal/toolgen/toolgen_test.go`

Verification commands:
- `go test ./internal/toolgen`
- `go test ./...`

## Risk
- Low: script remains offline and preserves the existing CLI.
- Low: temp stderr file cleanup is managed with a trap and is scoped to script
  execution.
- Residual risk: `go list` may emit useful warnings even with valid packages;
  this implementation re-emits them so operator diagnostics are preserved.
