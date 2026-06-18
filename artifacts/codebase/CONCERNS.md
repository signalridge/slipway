# Concerns

- Architectural pressure points: AI-facing prose can accidentally become
  pseudo-authority. The workflow skill explicitly says CLI gates and command
  semantics live in the CLI, so new handoff prose must preserve that boundary.
  Evidence: `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:10-12`,
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:66-70`.
- Brittle areas: `SessionStart` surfaces handoff path only; embedding or
  trusting handoff body content would expand hook output and weaken the narrow
  contract. Evidence: `cmd/session_start_hook.go:126-135`,
  `cmd/session_start_hook.go:166-170`.
- Migration traps: shared reference docs are copied only when SKILL.md names
  the doc. If adding a new shared skill-quality reference, either wire it into
  `sharedReferenceDocs` or use the existing checklist reference deliberately.
  Evidence: `internal/toolgen/toolgen.go:1633-1678`.
- Recheck routing: use `slipway next --json` and generated host skills for
  lifecycle routing; do not route from stale chat or `handoff.md` prose.
- Notes: user explicitly requested preserving useful skill guidance; prose
  cleanup should be local, high-ROI, and backed by tests.
