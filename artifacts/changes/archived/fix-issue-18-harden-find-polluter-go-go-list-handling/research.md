# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`
  - `internal/toolgen/toolgen_test.go`
- Dependency chains: toolgen fixture tests execute shipped skill scripts from
  `internal/tmpl/templates/skills/...`; generated skill surfaces depend on those
  templates staying executable and deterministic.
- Blast radius: low. The change affects package enumeration in one diagnostic
  helper script and fixture-contract tests for that helper.
- Constraints: preserve the script CLI and bash implementation; keep `go list`
  hard failures distinct from "no test packages found"; preserve external
  `_test` package discovery.

### Patterns
- Existing conventions: scripts emit `find-polluter-go.sh:` diagnostics to
  stderr and return non-zero for usage, pre-existing pollution, go-list failures,
  empty package sets, and detected polluters.
- Reusable abstractions: existing `runCommandInDir` fixture helper covers script
  execution in controlled temporary modules.
- Convention deviations: none required. The script already depends on bash
  arrays/process substitution, so a bash-specific temp stderr file is consistent.

### Risks
- Technical risks: low. Capturing stderr separately could hide useful go-list
  diagnostics unless they are re-emitted; the implementation re-emits stderr.
- Guardrail domains: none.
- Reversibility: simple file-level revert restores prior behavior.

### Test Strategy
- Existing coverage: `TestScriptFixtureContracts` already covers usage errors,
  go-list failure reporting, and external-test-only package enumeration.
- Infrastructure needs: a temp parent module with an empty child directory to
  reproduce the ancestor-`go.mod`/no-package case without writing `/tmp/go.mod`.
- Verification approach:
  - Reproduce the issue with the script in an empty child under a temp `go.mod`.
  - Assert `go list` hard failures still report `go list failed for ...`.
  - Assert `go: warning: "./..." matched no packages` is not treated as a
    package name.
  - Run `go test ./internal/toolgen`, then `go test ./...`.

## Alternatives Considered
- Accept either `go list failed` or `matched no packages` in the test only:
  cheap, but leaves the shipped script parsing stderr warnings as packages.
- Force the test into a module-free temp root: narrows the test environment, but
  does not cover the developer-machine failure mode from issue #18.
- Selected: capture `go list` stdout and stderr separately in the script, treat
  empty stdout as `no test packages found`, and add a fixture for the
  ancestor-module/no-package case. This fixes the real script robustness defect
  and makes the test deterministic.

## Unknowns
- Resolved: whether the issue reproduces without writing `/tmp/go.mod` ->
  yes, a temp parent module plus empty child reproduces the bad path.
- Resolved: whether external `_test` package enumeration must be preserved ->
  yes, existing fixture coverage asserts `example.com/polluter/polluter`.
- Remaining: None.

## Assumptions
- The script may continue requiring bash. Evidence: existing implementation uses
  bash arrays and process substitution in
  `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`.
- `go test ./internal/toolgen` is the targeted test signal. Evidence:
  `internal/toolgen/toolgen_test.go` owns `TestScriptFixtureContracts`.

## Canonical References
- `https://github.com/signalridge/slipway/issues/18`
- `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`
- `internal/toolgen/toolgen_test.go`
- `artifacts/codebase/TESTING.md`
- `artifacts/changes/fix-issue-18-harden-find-polluter-go-go-list-handling/intent.md`
