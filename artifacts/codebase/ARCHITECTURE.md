# Architecture

- Question: What seams must this change touch to add an engine-owned per-change
  handoff command without creating a second lifecycle authority?
- Runtime location: per-change runtime state lives under the git-common Slipway
  namespace. `state.ChangeHandoffPath(root, slug)` already defines the advisory
  handoff path as `ChangeDir(root, slug)/handoff.md`, so the new writer should
  reuse that path rather than introducing a parallel location. Evidence:
  `internal/state/local_runtime_paths.go:361-369`.
- Active-change resolution: command and hook entrypoints should reuse
  `resolveActiveChangeRef`, which resolves explicit `--change` first and then
  prefers the invoking git worktree's active binding through
  `state.FindActiveChangeForWorktree`. Evidence: `cmd/common.go:312-364`.
- CLI command surface: root command registration is in `cmd/root.go`, while new
  command implementation belongs in a focused `cmd/handoff.go` with companion
  command tests. The command must remain advisory and must not participate in
  gate/evidence/freshness evaluation.
- Hook surface: `slipway hook session-start` is implemented in
  `cmd/session_start_hook.go` and already emits an authoritative `next` block
  plus a bounded handoff presence/path summary. `slipway hook context-pressure`
  is implemented in `cmd/context_pressure_hook.go` and currently emits a generic
  workflow handoff nudge. Evidence: `cmd/session_start_hook.go:34-68`,
  `cmd/session_start_hook.go:131-142`, `cmd/context_pressure_hook.go:430-449`.
- Tool generation surface: adapter metadata, command registration, command-skill
  rendering, hook settings merging, and human invocation summaries are centralized
  in `internal/toolgen/toolgen.go`. Codex currently uses command skills and has
  no configured hook settings path, which makes generated `.codex/config.toml`
  support a new toolgen surface rather than a hook-only patch. Evidence:
  `internal/toolgen/toolgen.go:25-46`, `internal/toolgen/toolgen.go:87-102`,
  `internal/toolgen/toolgen.go:2454-2463`.
