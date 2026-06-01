# Decision

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Move agent metadata to typed Go registry
This would remove hidden markdown templates while preserving `agents.mappings`
and `manual_only` semantics. It minimizes behavioral change, but keeps an
agent abstraction that no public handoff consumes.

### Soft-deprecate `agents.mappings`
This would parse `agents:` and warn or ignore it. It has better compatibility
for old configs, but leaves dead configuration accepted by the runtime.

### Hard-delete agent config and hidden templates
This removes top-level `agents:` from the config schema, deletes the hidden
template directory, removes registry override behavior, and keeps public
handoff skill-first.

## Selected Approach
Use hard deletion. The user explicitly confirmed that configuration validation
semantics are not needed and asked to remove the config directly. The selected
approach is:

- reject top-level `agents:` during config parse with an actionable removed
  config error
- delete `ConfigAgents`, `Config.Agents`, serialization, validation, and tests
  for `agents.mappings`
- delete registry override logic and agent-status frontmatter parsing
- delete `internal/tmpl/templates/agents/*.md`, the embed directive, and
  `tmpl.AgentNames`
- keep `health` focused on generated host skill surfaces
- keep generated adapters from creating `.*/agents` directories

## Interfaces and Data Flow
- Removed input surface: `.slipway.yaml` top-level `agents:`.
- Removed internal data flow: `model.LoadConfig` -> `cfg.Agents.Mappings` ->
  `skill.LoadGovernanceRegistry` -> `Definition.AgentHint` override.
- Removed internal template flow: `tmpl.AgentNames` / `tmpl.Content("agents/...")`
  -> `configuredAgentStatus` and health template checks.
- Preserved public data flow: `next --json` -> `next_skill.name` -> caller
  derives `slipway-{name}/SKILL.md`.

## Rollout and Rollback
- Rollout is a normal code/test change with no data migration.
- Existing workspaces that still contain `agents:` must edit `.slipway.yaml`;
  this is intentional and should fail clearly at parse time.
- Rollback is a git revert restoring the config field, registry override,
  hidden templates, and associated tests.

## Risk
- Compatibility risk: old configs with `agents:` fail. Accepted by user
  direction.
- Regression risk: registry/health tests may encode old agent assumptions.
  Mitigation is focused test updates plus full `go test ./...`.
- Generated-surface risk: adapters must remain skill-first and no-agent. Keep
  existing no-`.*/agents` tests and next JSON no-`agent_hint` tests.
