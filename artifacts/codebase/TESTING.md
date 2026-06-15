# Testing

Re-authored for change
`eliminate-non-native-hook-and-skill-script-runtime-dependenc`.

## Existing Coverage

- `cmd/context_pressure_hook_test.go` covers the compiled
  `slipway hook context-pressure` behavior and should remain the model for
  testing hook behavior without executing generated shell scripts.
- `internal/tmpl/hooks_behavior_test.go` currently executes rendered bash hook
  templates. This is the coverage to retire or narrow to launcher-only contract
  tests.
- `internal/toolgen/adapter_contract_test.go` and
  `internal/toolgen/toolgen_test.go` freeze hook paths and settings commands;
  today they assert `bash "<path>.sh"` and must be updated to reject that
  contract.
- `internal/toolgen/toolgen_test.go` also runs template-side skill scripts via
  bash and Python for SARIF merge, action pinning, Go polluter tracing, variant
  scaffolding, and GitHub helpers. These tests must move to compiled
  `slipway tool` command behavior.
- `internal/toolgen/support_files_test.go` verifies support file copying,
  including `scripts/` payloads and shared `gh-common.sh`; it should instead
  assert generated skills no longer ship executable helper scripts.
- `internal/tmpl/templates_test.go` verifies rendered templates have no
  unexpanded variables and keeps hook templates compact.

## Gaps Closed By This Change

- Session-start hook behavior receives Go command tests for success, scoped
  worktree handoff paths, missing Slipway state, and fail-silent diagnostics.
- Hook settings tests reject legacy `bash`, `.sh` canonical commands, and
  script-specific settings.
- Launcher template tests cover only native binary dispatch and fail-silent
  behavior; lifecycle output is tested in `cmd`.
- Skill helper tests run compiled `slipway tool` commands without bash, Python,
  jq, or `gh` on PATH.
- GitHub helper tests use local HTTP test servers and token environment
  variables instead of the GitHub CLI.
- Template inventory tests assert no generated `skills/*/scripts/*` support
  payload remains for Slipway-owned helpers.

## Verification Plan

- Run focused command tests for new hook and tool subcommands.
- Run affected packages:
  `go test ./cmd ./internal/tmpl ./internal/toolgen`.
- Run repository verification:
  `gofmt -l`, `go test -count=1 ./...`, and `git diff --check`.
- Use current-worktree Slipway outputs (`status --json`, `validate --json`,
  `next --json --diagnostics`) after evidence refreshes.
