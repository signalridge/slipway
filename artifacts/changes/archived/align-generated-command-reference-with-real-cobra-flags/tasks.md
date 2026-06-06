# Tasks
## Project Context
- Tech Stack: Go
- Conventions: Slipway Agent Principles (CLAUDE.md). Fix in generator sources +
  regenerate; never hand-edit the gitignored generated tree.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Export `CommandArguments(id)` and backfill `commandRegistry[].Arguments` so every non-hidden Cobra flag is listed (next:`--no-auto-pass`; status:`--format,--hydrate,--hydrate-ref,--root,--stats`; review:`--format,--hydrate,--hydrate-ref`; validate:`--format`; repair:`--format`; health:`--format,--hydrate,--hydrate-ref`).
  - wave: 1
  - depends_on: []
  - target_files: [internal/toolgen/toolgen.go]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-02` Backfill core action flags in command body templates' `## Flags` sections (pivot `--reroute/--rescope`; done `--all-ready`; next `--no-auto-pass`; review `--all/--changed-only/--focus/--list-focuses`; validate `--focus/--list-focuses`).
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/templates/_partials/command-pivot-body.tmpl, internal/tmpl/templates/_partials/command-done-body.tmpl, internal/tmpl/templates/_partials/command-next-body.tmpl, internal/tmpl/templates/_partials/command-review-body.tmpl, internal/tmpl/templates/_partials/command-validate-body.tmpl]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-03` Audit and correct Cobra `--help` text (Short/Long + flag usage strings) for divergence from real behavior across the affected commands; record any genuine behavior-bug divergence as an out-of-scope note rather than changing logic.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/next.go, cmd/status.go, cmd/review.go, cmd/validate.go, cmd/repair.go, cmd/health.go]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-04` Redesign the Slipway entry skill: rewrite `description` toward task-side trigger language (fix the discovery paradox), clarify the three-layer boundary (entry vs `slipway:*` vs `slipway-*`), and keep its route table/handoff consistent with current CLI.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/skills/workflow/SKILL.md.tmpl, internal/toolgen/toolgen.go]
  - task_kind: code
  - covers: [REQ-004, REQ-008]

- [x] `t-05` Make the SessionStart hook surface "load the slipway skill" so an upstream discovery path points AT the entry skill (hook becomes a trigger, not a silent replacement).
  - wave: 2
  - depends_on: []
  - target_files: [internal/tmpl/templates/hooks/session-start.sh.tmpl]
  - task_kind: code
  - covers: [REQ-008]

- [x] `t-06` Align flag/usage citations in generated skill surfaces (entry `command-reference.md.tmpl`, `slipway-*` host SKILL.md templates) and the references/skill-index with the real flag set after t-01.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/skills/workflow/command-reference.md.tmpl]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-07` Align docs/ and README command/flag usage and examples with the real flag set (no phantom flags, no stale options).
  - wave: 2
  - depends_on: []
  - target_files: [docs/commands.md, docs/workflow.md, docs/operator-guide.md, docs/index.md, README.md]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-08` Add the reverse flag-contract guard: extend `cmd/template_flag_contract_test.go` to assert every non-hidden, non-help Cobra flag appears in `toolgen.CommandArguments(id)`, with a documented exemption list (`help`, `review:--artifact` unsupported-in-MVP). Fails closed in CI.
  - wave: 2
  - depends_on: [t-01, t-02]
  - task_kind: code
  - covers: [REQ-007]
  - target_files: [cmd/template_flag_contract_test.go]

- [x] `t-09` Regenerate (`slipway init --refresh`), run the `--help`-vs-generated-surface drift scan to zero missing flags, and confirm `go build ./...` + `go test ./cmd/... ./internal/toolgen/... ./internal/tmpl/...` green.
  - wave: 3
  - depends_on: [t-03, t-04, t-05, t-06, t-07, t-08]
  - target_files: [internal/toolgen/toolgen.go]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008]
