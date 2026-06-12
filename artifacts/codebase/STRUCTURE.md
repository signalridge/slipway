# Structure

Re-authored for change
`resolve-github-issue-184-add-gsd-style-automatic-subagent-di`
(GitHub issue #184).

- `internal/tmpl/templates/skills/wave-orchestration/`
  - `SKILL.md.tmpl`: primary host skill template for governed S2 execution.
  - `references/executor-dispatch-reference.md`: runtime-specific executor
    dispatch reference copied beside generated wave-orchestration skills.
- `internal/tmpl/`
  - `thin_host_content_test.go`: focused template/reference content tests.
  - `wave_isolation_content_test.go`: existing dispatch-contract coverage for
    test-authoring isolation and TDD sequencing.
  - `templates_test.go`: broader rendered-template contract tests.
- `internal/toolgen/`
  - `toolgen.go`: adapter registry and skill generation authority.
  - `toolgen_test.go`: generated tree contract tests, including existing
    wave-orchestration parallel-by-default assertions.
  - `support_files_test.go` and `testdata/skill_tree_inventory.codex.golden`:
    generated support-file inventory checks for Codex.
  - `surface_manifest.go` and `surface_manifest_test.go`: generated public
    surface inventory; relevant only if a new surface row is introduced.
- `docs/`
  - `ai-tools.md`: documents `docs/SURFACE-MANIFEST.json` regeneration when new
    public surface rows are added.
  - `operator-guide.md`: documents `slipway init --tools all --refresh` after
    template or command-contract changes.
