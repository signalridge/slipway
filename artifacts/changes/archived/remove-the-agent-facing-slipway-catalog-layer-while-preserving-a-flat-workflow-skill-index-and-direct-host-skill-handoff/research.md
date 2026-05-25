# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/toolgen/toolgen.go`: adapter generation, skill path helpers,
    workflow skill data, stale artifact cleanup, catalog route-card/support
    emission, and top-level catalog manifest emission.
  - `internal/engine/capability/export.go`: registry-to-markdown index
    rendering and default path shape.
  - `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`: generated
    workflow skill wording for runtime authority and handoff guidance.
  - `cmd/next_skill_view.go`: support/technique hint labels that currently
    expose `catalog:slipway/references/catalog/...` paths.
  - `internal/engine/capability/registry_b3.go`,
    `internal/engine/capability/registry_b5.go`, and matching
    `incident-response` / `threat-modeling` skill templates: export-only
    bindings that still name the retired `using-slipway-catalog` target.
  - Tests/golden fixtures under `internal/toolgen`,
    `internal/engine/capability`, `cmd`, and
    `internal/toolgen/testdata/skill_tree_inventory.codex.golden`.
- Dependency chains:
  - `slipway init` -> `internal/toolgen.generateForTool` -> generated
    `.codex/skills`, `.claude/skills`, `.opencode/skills`, etc.
  - `workflow/SKILL.md.tmpl` -> `renderStandaloneWorkflowSkill` ->
    `buildWorkflowSkillData` -> generated workflow `SKILL.md`.
  - `capability.DefaultRegistry` -> `renderCatalogSkill` for real exported
    host skills and currently -> `emitCatalogArtifacts` /
    `BuildCatalogManifestWithPaths` for the old catalog route layer.
- Blast radius: generated AI-facing skill artifacts and route hints only. The
  change must not alter lifecycle transition rules, change artifact authority,
  or command execution semantics.
- Constraints: `change.yaml` remains the lifecycle authority; the generated
  skill index is informational. Real executable procedure authority must stay
  in `.codex/skills/slipway-<name>/SKILL.md` and corresponding adapter paths.

### Patterns
- Existing conventions:
  - Toolgen centralizes adapter-specific path decisions in helpers such as
    `SkillPath` and generator-owned constants.
  - Generated support files for a real skill are emitted with
    `emitSkillSupportFiles`, rooted under that real skill directory.
  - Refresh cleanup is marker/allowlist oriented and should remove stale
    generated artifacts only where Slipway owns them.
- Reusable abstractions:
  - Keep `capability.BuildCatalogManifestWithPaths`, but repurpose it into a
    skill index renderer with direct host skill paths instead of catalog
    artifact paths.
  - Keep `renderCatalogSkill` for registry-owned real skills; the name
    "catalog" is internal Go vocabulary and does not need to appear in
    generated agent-facing paths.
  - Use `SkillPath(cfg, id)` as the direct handoff path for exported skills.
- Convention deviations: generated references should move from a top-level
  `.codex/skills/using-slipway-catalog.md` file into the workflow skill's own
  `references/skill-index.md` file. That is a path contract change and must be
  covered by tests.

### Risks
- Technical risks:
  - Medium: stale generated `using-slipway-catalog.md` or
    `slipway/references/catalog/**` files may survive `slipway init --refresh`
    unless cleanup explicitly removes the retired paths.
  - Medium: non-exported capability metadata may disappear from agent-facing
    docs. This is acceptable if the index records only actionable exported host
    skills, but tests should make the boundary explicit.
  - Low: wording changes in generated workflow skills could drift from CLI JSON
    behavior if they imply routing authority outside `next_skill.name`.
- Guardrail domains: `external_api_contracts`, because generated skill files
  are consumed by external AI hosts as a behavioral contract.
- Reversibility: safe to roll back by restoring the old generator paths/tests;
  no runtime state or user data migration is involved.

### Test Strategy
- Existing coverage:
  - `internal/toolgen/toolgen_test.go` already validates generated skill trees,
    generated workflow skill text, catalog artifacts, support-file copying,
    and stale cleanup behavior.
  - `internal/engine/capability/export_test.go` validates deterministic
    registry index rendering.
  - `cmd/next_skill_capability_hints_test.go` validates route hint labels.
  - `internal/toolgen/testdata/skill_tree_inventory.codex.golden` captures the
    Codex generated tree inventory.
- Infrastructure needs: no new test harness required; update existing focused
  tests and golden fixtures.
- Verification approach:
  - Focused tests: `go test ./internal/engine/capability ./internal/toolgen ./cmd`
  - Broad tests: `go test -timeout=20m ./... -count=1`
  - Build: `go build ./...`
  - Contract assertions: generated output lacks
    `using-slipway-catalog.md` and `slipway/references/catalog`, contains
    `slipway/references/skill-index.md`, and workflow wording points directly
    to host skill `SKILL.md` paths.

## Alternatives Considered
- Keep old catalog layer but rename files: simplest diff, but preserves the
  misleading agent-facing model and does not solve the user's concern about a
  separate catalog route system.
- Delete all index/reference surfaces: removes duplication, but loses useful
  audit/navigation context and makes `slipway/SKILL.md` less self-contained.
- Selected: remove the agent-facing catalog layer, generate one workflow-owned
  `references/skill-index.md`, point index rows directly at exported host skill
  paths, and clean stale retired files on refresh. This keeps navigation
  value while making real `slipway-<name>/SKILL.md` files the only execution
  authority.

## Unknowns
- Resolved: where is `using-slipway-catalog.md` emitted? ->
  `internal/toolgen/toolgen.go` writes `CatalogManifestPath(cfg)` after
  `BuildCatalogManifestWithPaths`.
- Resolved: where is `slipway/references/catalog/**` emitted? ->
  `emitCatalogArtifacts`, `CatalogArtifactPath`, `catalogSupportRootPath`, and
  `emitCatalogSupportFiles` in `internal/toolgen/toolgen.go`.
- Resolved: where are old path assumptions tested? ->
  `internal/toolgen/toolgen_test.go`,
  `internal/engine/capability/export_test.go`,
  `cmd/next_skill_capability_hints_test.go`, and
  `internal/toolgen/testdata/skill_tree_inventory.codex.golden`.
- Remaining: None.

## Assumptions
- Generated workflow skill references may keep a compact skill index as
  documentation only. Evidence: user explicitly asked to preserve indexing but
  flatten `.codex/skills/slipway/references/catalog` into
  `.codex/skills/slipway/references`.
- Non-exported capability metadata does not need an agent-facing pseudo-skill
  path. Evidence: the desired model names real `slipway-<name>/SKILL.md` files
  as execution authority and treats index content as reference.
- Runtime capability registry names can keep internal "catalog" naming if the
  generated external surface no longer exposes catalog as a route layer.
  Evidence: the user's objection was specifically generated `.codex/skills`
  paths and files.

## Canonical References
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/changes/remove-the-agent-facing-slipway-catalog-layer-while-preserving-a-flat-workflow-skill-index-and-direct-host-skill-handoff/intent.md`
- `internal/toolgen/toolgen.go`
- `internal/engine/capability/export.go`
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`
- `cmd/next_skill_view.go`
- `internal/toolgen/toolgen_test.go`
- `internal/engine/capability/export_test.go`
- `cmd/next_skill_capability_hints_test.go`
- `internal/toolgen/testdata/skill_tree_inventory.codex.golden`
