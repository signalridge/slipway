# Architecture

Re-authored for change
`eliminate-non-native-hook-and-skill-script-runtime-dependenc`.

Question: which Slipway adapter and helper seams must move from generated
shell/Python payloads into compiled Slipway commands while preserving
cross-platform host integration?

## Affected Seams

- `cmd/root.go` registers the public and hidden command tree. It currently
  includes `makeHookCmd`; this change adds compiled helper entrypoints under
  the Slipway binary instead of asking generated adapters to execute script
  payloads directly.
- `cmd/context_pressure_hook.go` already owns the compiled
  `slipway hook context-pressure` behavior. It is the precedent for hook
  behavior living in Go while generated files remain launch adapters only.
- A new hook command owns `slipway hook session-start --tool <tool>`. It must
  replace the lifecycle/path/JSON logic currently embedded in
  `internal/tmpl/templates/hooks/session-start.sh.tmpl`.
- A new tool command family owns supported skill helpers such as SARIF merge,
  action pinning, variant query scaffolding, Go polluter tracing, and GitHub PR
  review/check helpers.
- `internal/toolgen/toolgen.go` owns adapter registry data, hook file emission,
  support-file copying, and hook settings merge. Its current registry stores
  `.sh` paths and its settings merge registers `bash "<hook>.sh"` commands.
- `internal/tmpl/templates/hooks/` owns rendered hook launcher templates. After
  this change these templates should contain only platform-native binary
  dispatch logic, not lifecycle behavior.
- `internal/tmpl/templates/skills/` owns generated skill instructions and
  optional support payloads. Executable `scripts/` payloads are no longer a
  valid generated runtime surface for supported Slipway helpers.

## Dependency Flow

Generated host adapters are produced by `toolgen` from embedded templates. Tool
settings invoke hook commands, which should now dispatch to the compiled
Slipway binary. Hook commands can query current worktree lifecycle state through
existing command/progression helpers and emit host-specific hook output.

Skill instructions are also produced by `toolgen`. Where a skill needs a
supported helper, the instruction points to `slipway tool <helper>`; the helper
logic runs inside the binary and uses the standard library plus explicit domain
tools only when the task inherently requires them, for example `go test` inside
the Go polluter helper.

## Constraints And Invariants

- Generated settings must not canonically invoke `bash`, `.sh`, Python, jq, or
  `gh` for Slipway-owned hooks.
- Platform-specific launchers are allowed when generated for the host platform
  and kept thin: locate/execute `slipway`, pass stdin/stdout through, and
  fail-silent for automatic hooks.
- Hook launchers must not duplicate lifecycle, path, JSON parsing, handoff, or
  context-pressure business logic.
- Manual `slipway tool` helpers must fail explicitly; only automatic hooks are
  allowed to no-op when the binary is unavailable.
- No backward-compatibility path is required for legacy generated
  `bash ".*/hooks/*.sh"` settings.
- Existing generated skill references and support files must be refreshed from
  `internal/tmpl/templates`; checked-in generated `.claude` or `.codex` output
  is not the source of truth.
