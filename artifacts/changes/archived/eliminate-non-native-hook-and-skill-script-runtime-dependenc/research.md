# Research

## Alternatives Considered

### Architecture

Option A: direct settings commands only.

- Register `slipway hook session-start --tool <tool>` and
  `slipway hook context-pressure` directly in settings.
- Upside: smallest generated surface and no launcher files.
- Downside: host tools surface command-not-found failures directly when
  `slipway` is unavailable, which is too brittle for automatic hooks.

Option B: compiled behavior with platform-native thin launchers. Selected.

- Keep all behavior in Go commands and generate native launcher files only to
  locate/execute the Slipway binary, pass through stdin/stdout, and fail-silent
  for automatic hooks.
- On Windows, render PowerShell or CMD launchers; on POSIX, render POSIX shell
  launchers. The launcher is not an alternate product implementation.
- This matches the user clarification that maintaining PowerShell/native
  adapters is acceptable when the platform renderer owns them and business
  logic does not drift across languages.

Option C: keep existing scripts and add health warnings.

- Upside: smaller immediate code change.
- Downside: leaves bash/Python/jq payloads and unstructured `gh` assumptions in
  generated helper paths, so Windows and minimal installations remain
  second-class. Rejected.

### Patterns

- Use Cobra subcommands for hidden automatic hooks and visible manual helper
  commands, following the existing `cmd/context_pressure_hook.go` shape.
- Keep helper behavior in the Slipway binary where possible. GitHub helpers use
  a selected authenticated backend: `gh` when available, token-backed API when
  `gh` is unavailable, and fail-closed remediation when neither exists. Local
  SARIF/YAML-like helpers use file IO and deterministic JSON processing.
- Treat generated launcher templates as adapter code, not domain logic. Tests
  should assert they contain only binary dispatch, no lifecycle or helper
  algorithms.
- Refresh generated support trees by deleting stale `scripts/` output instead
  of preserving old payloads.

### Risks

- Hook settings may retain stale `bash "<hook>.sh"` entries after refresh unless
  the merge code removes Slipway-owned legacy commands.
- Moving helper output to Go may change JSON ordering, SARIF merge shape, or
  GitHub error wording. Command tests need to encode the intended contract.
- PowerShell/CMD quoting can diverge from POSIX. The launchers must keep their
  argument set fixed and small.
- GitHub helper behavior can hide authentication problems if it silently falls
  through unauthenticated API paths. Skill docs and tests must require an
  explicit authenticated backend and fail closed when none is available.
- Some task workflows still require domain tools, for example `go test`,
  CodeQL, or Semgrep. Documentation must distinguish those from Slipway helper
  runtime dependencies.

### Test Strategy

- Add Go command tests for `slipway hook session-start --tool <tool>` covering
  normal output, scoped handoff path, missing state, and diagnostic/no-op cases.
- Keep `slipway hook context-pressure` tests in Go and narrow template tests to
  launcher dispatch.
- Replace script-execution fixture tests with `slipway tool` command tests for
  SARIF merge, action pinning, Go polluter tracing, variant scaffolding, and
  GitHub helper credential/dry-run behavior.
- Use `httptest.Server` and injected GitHub CLI runners for GitHub helpers so
  tests do not require network, real `gh` authentication, jq, Python, or real
  credentials.
- Update toolgen contract tests so generated registered hooks do not contain
  `bash`, `.sh` canonical commands, Python, jq, or `gh` helper paths.
- Update template inventory tests so `skills/*/scripts/*` payloads are absent
  from generated Slipway skill surfaces.

### Options

The chosen implementation is Option B because it keeps platform integration
pragmatic without fragmenting behavior. PowerShell/native launchers solve host
startup ergonomics; Go subcommands solve correctness, testability, and runtime
dependency control.

## Unknowns

None. The hook and skill script inventory was completed from current source
templates and tests.

## Assumptions

- The compiled `slipway` binary is the only Slipway-provided runtime dependency
  users should need for generated hooks. Manual skill helper commands may use
  explicit authenticated backends or domain tools when that is the correct
  backend for the workflow.
- Automatic hook failure should not interrupt the host AI tool; manual helper
  failure should be explicit and non-zero.
- Existing generated script payloads are helper behavior rather than immutable
  public APIs; the user explicitly said backward compatibility is not required.
- Platform-rendered PowerShell/CMD launchers are acceptable when generated and
  thin.

## Canonical References

- `cmd/context_pressure_hook.go` for existing compiled hook behavior and
  fail-silent hook semantics.
- `cmd/root.go` for root command registration.
- `internal/toolgen/toolgen.go` for adapter registry, hook generation, settings
  merge, support payload copy, and deterministic writes.
- `internal/tmpl/templates/hooks/session-start.sh.tmpl` for current
  session-start behavior that must move into Go.
- `internal/tmpl/templates/hooks/context-pressure-post-tool-use.sh.tmpl` for
  the existing thin dispatcher pattern.
- `internal/tmpl/templates/skills/*/scripts/*` for the current non-native
  helper inventory.
- `internal/toolgen/toolgen_test.go`, `internal/toolgen/adapter_contract_test.go`,
  `internal/toolgen/support_files_test.go`, and
  `internal/tmpl/hooks_behavior_test.go` for the tests that currently enforce
  script-based contracts.
