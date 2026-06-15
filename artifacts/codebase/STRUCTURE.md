# Structure

Re-authored for change
`eliminate-non-native-hook-and-skill-script-runtime-dependenc`.

- `cmd/`
  - `root.go`: registers `hook` and the new `tool` command surface.
  - `context_pressure_hook.go`: existing compiled PostToolUse hook behavior.
  - `session_start_hook.go`: new compiled SessionStart hook behavior.
  - `tool_*.go`: new compiled skill helper implementations and tests.
- `internal/toolgen/`
  - `toolgen.go`: adapter registry, hook launcher emission, hook settings merge,
    skill support copying, and deterministic write modes.
  - `adapter_contract_test.go`: frozen generated adapter hook contract.
  - `toolgen_test.go`: generated settings, hook, skill, and helper behavior
    contracts.
  - `support_files_test.go`: support payload inventory and stale cleanup.
- `internal/tmpl/`
  - `templates.go`: embedded template set. It currently embeds
    `templates/skills/*/scripts/*`; that directive should be removed once
    helper scripts are deleted.
  - `templates/hooks/`: generated platform hook launcher templates.
  - `templates/skills/`: generated skill instructions and references. Runtime
    helper references should use `slipway tool ...` commands.
- `artifacts/changes/eliminate-non-native-hook-and-skill-script-runtime-dependenc/`
  - `intent.md`: approved no-compat scope and launcher boundary.
  - `research.md`: discovery, alternatives, selected architecture, and test
    strategy.
  - `requirements.md`, `decision.md`, `tasks.md`, `assurance.md`: plan bundle
    to author after research evidence is recorded.
