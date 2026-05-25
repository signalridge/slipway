# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI, generated Codex skills
- Languages: Go
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Conventions: 

## Summary
Remove the agent-facing Slipway catalog layer while preserving a flat workflow skill index and direct host skill handoff.
## Complexity Assessment
complex
Rationale: this changes generated Codex skill surfaces, docs/templates, and
tests that define the external handoff contract for AI agents.

## Guardrail Domains
external_api_contracts

## In Scope
- Remove the generated top-level `.codex/skills/using-slipway-catalog.md`
  surface.
- Remove the agent-facing `.codex/skills/slipway/references/catalog/**`
  route-card/support tree.
- Preserve indexing as a workflow-owned reference under
  `.codex/skills/slipway/references/skill-index.md`.
- Update `slipway/SKILL.md` generation so governed execution points directly
  from `slipway next --json` / `next_skill.name` to
  `.codex/skills/slipway-<name>/SKILL.md`.
- Keep concrete procedure, checklist, overlay, and script content only under
  the corresponding real `slipway-<name>/` host or technique skill directory.
- Update tests and golden/generated assertions to prevent reintroducing the
  old catalog route layer.

## Out of Scope
- Do not remove the primary `.codex/skills/slipway/SKILL.md` workflow entry.
- Do not remove real exported `slipway-<name>/SKILL.md` host/technique skills.
- Do not change lifecycle state transitions, governance gate semantics, or CLI
  command behavior except where generated skill paths/text require alignment.
- Do not introduce a new alternate routing mode or compatibility flag for the
  old catalog layout.

## Constraints
- Keep generated Codex skills deterministic.
- Keep the index informational only; it must not become a second execution
  authority.
- Preserve direct host handoff semantics for AI agents and CLI JSON consumers.

## Acceptance Signals
- Generated Codex skill output no longer contains
  `.codex/skills/using-slipway-catalog.md`.
- Generated Codex skill output no longer contains
  `.codex/skills/slipway/references/catalog/**`.
- Generated workflow references include
  `.codex/skills/slipway/references/skill-index.md`.
- The generated workflow skill no longer instructs agents to use a catalog
  artifact path and instead names direct host skill handoff.
- Focused toolgen/capability tests and broad Go tests pass.

## Open Questions

## Intake Research Notes
Resolved during intake research:
- `internal/toolgen/toolgen.go` emits the top-level catalog manifest via
  `CatalogManifestPath` / `BuildCatalogManifestWithPaths`, catalog route cards
  via `emitCatalogArtifacts` / `renderCatalogArtifact`, and copied support
  roots via `emitCatalogSupportFiles` / `catalogSupportRootPath`.
- `internal/engine/capability/export.go` owns the generated index text and
  default path shape.
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl` references
  `using-slipway-catalog.md` and catalog artifact handoff guidance.
- `internal/toolgen/toolgen_test.go`,
  `internal/engine/capability/export_test.go`,
  `cmd/next_skill_capability_hints_test.go`, and
  `internal/toolgen/testdata/skill_tree_inventory.codex.golden` encode the old
  catalog path contract.

No unresolved user-scope questions remain.

## Deferred Ideas
- Formal promotion of currently non-exported capability metadata into real
  host skills is deferred unless required to preserve existing generated
  behavior.

## Approved Summary
Confirmed from the user discussion on 2026-05-25: remove the external catalog
concept from generated Codex skill artifacts, flatten workflow-level references
under `.codex/skills/slipway/references`, preserve a simple skill index there,
and make real `slipway-<name>/SKILL.md` files the only executable skill
authority for host/technique procedures.
