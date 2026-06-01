# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Remove obsolete internal agent markdown templates and the now-unused `agents`
configuration surface. Slipway should route governed work by host skills, not
by hidden agent definitions or configurable agent mappings.
## Complexity Assessment
complex
Rationale: this is a meta/change-surface cleanup touching config parsing,
registry loading, health diagnostics, embedded templates, generated-surface
tests, and documentation contracts.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Remove `.slipway.yaml` `agents:` / `agents.mappings` as a supported config
  surface.
- Remove runtime use of configured agent mappings from governance registry
  loading.
- Remove `internal/tmpl/templates/agents/*.md`, the `templates/agents` embed,
  and helper APIs that only exist to load those hidden markdown files.
- Remove health checks that validate built-in agent templates or configured
  agent mappings.
- Keep the external handoff contract skill-first: `next --json` returns
  `next_skill.name`; callers derive `slipway-{name}/SKILL.md`.
- Keep adapter generation from creating `.*/agents` project directories.
- Update tests and docs for the removed config/template surface.

## Out of Scope
- Do not add Claude-only plugin/subagent packaging in this change.
- Do not introduce a new replacement agent registry.
- Do not change workflow state ordering, artifact schemas, or command
  invocation syntax.
- Do not broaden generated host skill content beyond what is required to remove
  stale agent references.

## Constraints
- Preserve generated-surface compatibility for Claude, Codex, Cursor, Gemini,
  and OpenCode.
- Treat hidden agent markdown as removed, not migrated to another hidden
  metadata carrier.
- Existing project-local `.*/agents` directories remain outside Slipway's
  generated contract.

## Acceptance Signals
- Parsing a config containing top-level `agents:` fails with an actionable
  unsupported-config error.
- `rg "ConfigAgents|Agents\\.Mappings|AgentNames|templates/agents|Content\\(\"agents/|agent_status|manual_only|governance_mapped" internal cmd`
  has no stale runtime references.
- Focused tests for config parsing, registry loading, health, templates, and
  tool generation pass.
- `go test ./...` passes.
- `go build ./...` passes.

## Open Questions
- None.

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
- 2026-06-01T07:12:48Z: User confirmed the configuration validation semantics
  are no longer needed and explicitly requested removing the config surface
  directly. The approved direction is to eliminate the agent abstraction from
  runtime config and hidden templates, while preserving skill-based governed
  handoff and generated skill surfaces.
