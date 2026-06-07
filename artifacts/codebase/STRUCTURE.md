# Structure

- Directory layout: cmd/, docs/, internal/
- Entry points: README.md, go.mod, main.go
- Generated versus handwritten boundaries:
  - `internal/tmpl/templates/skills/*/SKILL.md*` are handwritten template
    sources for generated agent surfaces.
  - `internal/tmpl/templates/skills/*/references/*` hold lazy-loaded details
    that should not be duplicated in host skill bodies.
  - `internal/toolgen/*.go` renders templates into generated runtime surfaces.
  - `artifacts/changes/*` and `artifacts/codebase/*` are governed context and
    evidence artifacts, not runtime code.
- Ownership hints:
  - Goal verification behavior instructions live in
    `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`.
  - Worktree preflight behavior instructions live in
    `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`.
  - Wave execution dispatch instructions live in
    `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` plus
    `internal/tmpl/templates/skills/wave-orchestration/references/`.
- Notes:
  - This change is scoped to generated governance skill context behavior and
    template tests; it does not touch `slipway new` create-time behavior.
