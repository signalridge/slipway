# Conventions

- Naming: generated command/skill surfaces use stable command IDs and
  `slipway-<name>` skill paths. Evidence:
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:72-80`.
- File organization: authored templates stay under `internal/tmpl/templates`;
  shared references are copied only when a consuming skill points at them.
  Evidence: `internal/toolgen/toolgen.go:1633-1678`.
- Error handling: hook surfaces fail silent where required and should not block
  user sessions; SessionStart writes a bounded wrapper and diagnostics instead
  of embedding handoff body content. Evidence: `cmd/session_start_hook.go:138-170`.
- Configuration: root CLI command descriptions come from the toolgen registry
  to keep CLI help and adapter surfaces aligned. Evidence: `cmd/root.go:13-15`.
- State management: governed lifecycle state and evidence freshness belong to
  Slipway CLI/runtime; template prose must not tell agents to hand-edit
  verification YAML or infer lifecycle state from prose.
- Notes: high-ROI token cleanup should remove repetition or contradiction only
  in touched templates; preserve project knowledge and contract tokens.
