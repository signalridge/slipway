# Architecture

- Module responsibilities: CLI behavior lives in `cmd/`; generated host
  content is authored in `internal/tmpl/templates/`; adapter emission and
  cross-host assertions live in `internal/toolgen`. Evidence: `cmd/root.go:28-90`,
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:1-114`,
  `internal/toolgen/toolgen_test.go:631-666`.
- Dependency flow: workflow and command surfaces route agents into the CLI;
  they are not lifecycle authority. The workflow skill states that the CLI owns
  transitions, gates, and command semantics. Evidence:
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:10-12`,
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:66-70`.
- Coupling hotspots: `SessionStart` currently surfaces only handoff presence
  and path, not body content; context pressure currently says to preserve a
  handoff without defining its authoring contract. Evidence:
  `cmd/session_start_hook.go:126-135`,
  `cmd/context_pressure_hook.go:435-440`.
- Current change blast radius: expected changes should be limited to authored
  templates, hook wording if needed, artifact/checklist templates, and tests
  that pin generated AI-facing contracts. Evidence:
  `internal/tmpl/templates_test.go:53-71`,
  `internal/tmpl/templates_test.go:586-612`,
  `cmd/context_pressure_hook_test.go:68-117`.
- Notes: do not add a new lifecycle state or treat `handoff.md` as evidence;
  this belongs in agent-facing guidance and regression tests.
