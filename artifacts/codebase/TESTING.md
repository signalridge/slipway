# Testing

- Question: What proof is needed for an advisory per-change handoff command,
  hook surfacing, and generated Codex hook config?
- Handoff artifact tests should cover creation, refresh, monotonic generation,
  narrative preservation, section updates, staleness derived from lifecycle-event
  timestamps, and absence of lifecycle state/substep or next_skill/next_command
  in the machine header and `--brief` descriptor. Planned targets:
  `internal/state/handoff_test.go`, `cmd/handoff_test.go`.
- Resolver and no-op behavior should be command-tested: slug-free active
  worktree resolution, explicit `--change`, and graceful no-op when no active
  change is bound. Planned target: `cmd/handoff_test.go`.
- Hook tests should assert the existing context-pressure hook points to
  `slipway handoff write`, SessionStart emits only a bounded pointer after the
  authoritative next block, ambiguous/root contexts never dump handoff bodies or
  focus text, and Codex-compatible hook output uses
  `hookSpecificOutput.additionalContext`. Planned targets:
  `cmd/context_pressure_hook_test.go`, `cmd/session_start_hook_test.go`.
- Toolgen tests should update command registry counts and generated command-skill
  expectations for `handoff`, assert `.codex/config.toml` hook generation uses
  Codex's `[[hooks.<Event>]]` shape, assert refresh/idempotence behavior, and
  update the current negative Codex config tests. Planned target:
  `internal/toolgen/toolgen_test.go`.
- Documentation/contract tests should keep README command contracts and init
  output honest, including the trust caveat that generated Codex hooks are inert
  until repo and hook trust are granted. Planned targets: `README.md`,
  `cmd/init_test.go`, `internal/toolgen/toolgen_test.go`.
- Minimum final verification remains `go build ./...`, `go vet ./...`,
  `gofmt -s -l`, and `go test ./...`; package-focused tests should be run
  earlier around `internal/state`, `cmd`, and `internal/toolgen` while the waves
  are implemented.
