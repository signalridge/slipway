# Research

## Research Findings

### Architecture
- Affected modules: config parsing/serialization in `internal/model/config.go`,
  governance registry loading in `internal/engine/skill/registry_loader.go`,
  registry defaults in `internal/engine/skill/skill.go`, health diagnostics in
  `cmd/health.go`, template embedding in `internal/tmpl/templates.go`, and
  tests in `cmd`, `internal/model`, `internal/engine/skill`,
  `internal/bootstrap`, `internal/tmpl`, and `internal/toolgen`.
- Dependency chains: `.slipway.yaml` is parsed by `model.LoadConfig`; registry
  loading currently reads `cfg.Agents.Mappings` and can mutate `Definition.AgentHint`;
  health calls `skill.LoadGovernanceRegistry` and separately checks
  `tmpl.AgentNames()` plus `tmpl.Content("agents/<name>.md")`; template content
  is embedded through `internal/tmpl/templates.go`.
- Blast radius: low-to-medium meta surface. The change removes an internal
  config/template path but must preserve public skill handoff and generated
  adapter surfaces.
- Constraints: `next --json` already hides `agent_hint`; external callers are
  expected to derive `slipway-{name}/SKILL.md` from `next_skill.name`. README
  and `docs/ai-tools.md` document skills/commands surfaces, not project-local
  agent surfaces.

### Patterns
- Existing conventions: configuration parsing is explicit in
  `ParseConfigYAML`, with recognized top-level keys decoded by name and unknown
  keys preserved through `UnknownTopLevel`. Config validation returns
  actionable path-like errors.
- Existing conventions: generated skill surfaces are built under tool-specific
  `SkillsDir` values through `toolgen.SkillPath`; tests assert `.*/agents`
  directories are not generated.
- Reusable abstractions: remove a config surface by deleting the typed field,
  parse branch, validation/serialization, and tests; keep unsupported legacy
  config explicit by rejecting top-level `agents:` during parse.
- Convention deviations: none required. This can stay within existing config,
  registry, health, template, and test patterns.

### Risks
- Technical risks: medium compatibility risk if existing users still have
  `agents:` in `.slipway.yaml`; this is intentional per user direction and
  should fail clearly rather than silently ignore.
- Technical risks: medium test churn across CLI/registry/bootstrap tests that
  currently use `cfg.Agents.Mappings` fixtures.
- Technical risks: low generated-surface risk; `.*/agents` non-generation tests
  should remain.
- Guardrail domains: none. This does not alter auth, secrets, PII, financial
  flows, schema migration, irreversible operations, or external API contracts.
- Reversibility: safe. Reverting restores the removed config/template path.

### Test Strategy
- Existing coverage: model config tests cover config parsing/serialization;
  skill registry tests cover default registry and overlay parsing; health tests
  cover agent-contract findings; template tests cover embedded template access;
  toolgen/init tests cover generated skill and no-agent-surface contracts.
- Infrastructure needs: no new test helpers required.
- Verification approach: add/adjust config parse tests for rejected top-level
  `agents:`; remove registry/health tests for configured agent mappings; keep
  `next --json` no-`agent_hint` tests where they still assert public contract;
  run focused packages followed by `go test ./...` and `go build ./...`.

## Alternatives Considered
- Keep hidden agent metadata but move from markdown to Go typed registry:
  preserves behavior but retains an agent abstraction that no public handoff
  consumes. Rejected because the user explicitly said the configuration
  validation semantics are no longer needed.
- Soft-deprecate `agents.mappings`: parse but ignore or warn. Better backward
  compatibility, but leaves a dead config surface and makes invalid old config
  appear accepted. Rejected because the user asked to directly remove the
  configuration.
- Selected: hard-delete the `agents` config surface and hidden agent markdown
  templates, with clear parse failure for top-level `agents:`. This best aligns
  code shape with the current public contract: governed handoff is by skill.

## Unknowns
- Resolved: Should `agents.mappings` validation semantics be preserved? -> No.
  User explicitly confirmed the config is not needed and should be removed.
- Remaining: None.

## Assumptions
- `AgentHint` can be removed entirely if no consumers remain after deleting
  config override and health agent validation. Evidence:
  `rg "AgentHint|agent_hint|AgentHintForSkillInRegistry" internal cmd docs README.md`.
- Project-local `.*/agents` non-generation remains a valid adapter contract.
  Evidence: `cmd/init_test.go` and `internal/toolgen/toolgen_test.go` assert no
  `.claude/agents`, `.codex/agents`, `.cursor/agents`, `.gemini/agents`, or
  `.opencode/agents` directories are generated.
- The public command contract remains skill-first. Evidence: README and
  `docs/ai-tools.md` list `.*/skills/slipway-*/SKILL.md` surfaces, while
  generated workflow guidance says not to look for retired agent fields.

## Canonical References
- `artifacts/changes/remove-obsolete-internal-agent-markdown-templates-and-keep-a/intent.md`
- `internal/model/config.go`
- `internal/engine/skill/registry_loader.go`
- `internal/engine/skill/skill.go`
- `cmd/health.go`
- `internal/tmpl/templates.go`
- `internal/tmpl/templates/agents/`
- `cmd/init_test.go`
- `internal/toolgen/toolgen_test.go`
- `README.md`
- `docs/ai-tools.md`
