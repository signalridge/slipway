# Architecture

- Module responsibilities:
  - `internal/tmpl/templates/skills/` is the source of truth for generated
    governance skill surfaces. Hand-editing `.codex/`, `.claude/`, `.cursor/`,
    or other generated copies is out of bounds.
  - `internal/toolgen/toolgen.go` owns which template-backed skills are exported
    for each supported tool and how generated runtime surfaces are laid out.
  - `internal/tmpl/templates_test.go` and focused `internal/tmpl/*_test.go`
    files protect generated-surface text contracts.
- Dependency flow:
  - Template files embed shared partials from `internal/tmpl/templates/_partials`
    where needed, then `toolgen` renders those files into per-tool skills and
    prompt surfaces.
  - The lifecycle engine routes by `next_skill.name`; the generated skill text
    instructs the host AI how to execute the stage without changing engine gates.
- Coupling hotspots:
  - `goal-verification` is closeout-adjacent and produces high-risk safety
    baseline references; any context optimization must preserve fail-closed
    evidence requirements.
  - `worktree-preflight` owns baseline proof before execution; it must record
    required worktree references while avoiding long baseline output in the
    main host.
  - `wave-orchestration` already dispatches task executors, but the host text
    still asks the coordinator to read broad codebase-map files before dispatch.
- Current change blast radius:
  - Generated governance skill templates and tests for template contracts.
  - No lifecycle engine gate weakening and no generated output hand edits.
- Notes:
  - Source references: `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`,
    `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`,
    `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`,
    `internal/toolgen/toolgen.go`, `internal/tmpl/templates_test.go`.
