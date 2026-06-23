# Structure

- Directory layout: `cmd/` owns CLI commands and hooks; `internal/tmpl/`
  owns authored templates; `internal/toolgen/` emits adapter surfaces; governed
  artifacts for this change live under `artifacts/changes/<slug>/`.
  Evidence: `cmd/root.go:28-90`, `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:1-114`,
  `internal/toolgen/toolgen_test.go:631-666`.
- Entry points: `main.go` delegates process exit behavior to `cmd.Execute`;
  root help groups include lifecycle, discovery, situational, helper,
  diagnostics, and setup commands. Evidence: `main.go:10-17`,
  `cmd/root.go:28-90`.
- Generated versus handwritten boundaries: edit template sources under
  `internal/tmpl/templates/...`, not generated `.codex/skills` or `.claude`
  outputs. Generated-surface contracts are pinned by `internal/tmpl` and
  `internal/toolgen` tests. Evidence: `internal/tmpl/templates_test.go:586-612`,
  `internal/toolgen/toolgen_test.go:631-666`.
- Ownership hints: workflow entry guidance belongs in
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`; command-specific
  guidance belongs in `_partials/command-*.tmpl`; shared review checklist
  guidance belongs in `skills/_shared/references/checklist-quality.md`.
  Evidence: `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:72-80`,
  `internal/tmpl/templates/_partials/command-run-body.tmpl:42-48`,
  `internal/tmpl/templates/skills/_shared/references/checklist-quality.md:1-6`.
- Notes: codebase-map docs are advisory and should stay bounded to this
  change's surfaces.
- New state helper files for this change should live under `internal/state/`
  beside other git-runtime path and state helpers. The plan's
  `internal/state/handoff.go` and `internal/state/handoff_test.go` targets are
  consistent with the existing `ChangeHandoffPath` runtime-path owner in
  `internal/state/local_runtime_paths.go`.
- `cmd/handoff.go` should own Cobra command wiring and presentation for
  `slipway handoff write/show`; hook handlers should call the command or shared
  command-owned logic rather than parsing handoff files directly.
- Codex project hook generation is structurally a toolgen concern. Existing
  tests currently assert that fresh Codex generation does not create
  `.codex/config.toml`; this change must update those contracts deliberately
  rather than leaving stale negative assertions in place. Evidence:
  `cmd/init_test.go:73-75`, `internal/toolgen/toolgen_test.go:1589-1590`,
  `internal/toolgen/toolgen_test.go:2018-2026`.
