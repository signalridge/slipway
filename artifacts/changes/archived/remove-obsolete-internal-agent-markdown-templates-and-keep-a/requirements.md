# Requirements

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Remove the agent mapping configuration surface
REQ-001: Slipway MUST stop supporting `.slipway.yaml` top-level `agents:` and `agents.mappings`.

#### Scenario: Legacy agents config is rejected
GIVEN a `.slipway.yaml` file contains top-level `agents:`
WHEN Slipway parses the config
THEN parsing fails with an actionable error explaining that `agents` configuration has been removed and governed handoff is skill-based.

### Requirement: Remove registry override behavior
REQ-002: Governance registry loading MUST NOT read or apply configured agent mappings.

#### Scenario: Registry uses Go-owned defaults
GIVEN a workspace has generated host skills
WHEN `skill.LoadGovernanceRegistry` loads definitions
THEN routing metadata comes from the Go registry and no config-driven agent override is applied.

### Requirement: Remove hidden agent markdown templates
REQ-003: Slipway MUST remove `internal/tmpl/templates/agents/*.md`, the corresponding `go:embed` directive, and helper APIs/tests that only exist to load those files.

#### Scenario: Template package has no agent template surface
GIVEN the template package is built
WHEN tests and static searches inspect embedded templates
THEN there is no `templates/agents` embed, no `tmpl.AgentNames`, and no `tmpl.Content("agents/...")` runtime path.

### Requirement: Remove agent-template health validation
REQ-004: `slipway health` MUST stop validating built-in agent templates or configured agent mappings.

#### Scenario: Health checks host skill surfaces only
GIVEN a generated adapter workspace is missing a required host skill surface
WHEN `slipway health --json` runs
THEN health reports the missing `slipway-<skill>/SKILL.md` surface.

#### Scenario: Project-local agent files are outside the contract
GIVEN `.claude/agents/slipway-planner.md` is absent
WHEN `slipway health --json` runs
THEN health does not report an agent-surface finding for that path.

### Requirement: Preserve skill-first public handoff
REQ-005: `next --json` and generated workflow guidance MUST continue using `next_skill.name` and host `slipway-{name}/SKILL.md` surfaces, without exposing `agent_hint` or agent definition paths.

#### Scenario: Next JSON does not expose retired agent fields
GIVEN a governed change is queried with `next --json`
WHEN the next skill payload is decoded
THEN it includes the skill name and excludes `agent_hint` and `agent_definition_path`.

### Requirement: Preserve no-agent generated adapter contract
REQ-006: Slipway adapter generation MUST continue avoiding `.*/agents` project directories for Claude, Codex, Cursor, Gemini, and OpenCode.

#### Scenario: Adapter generation does not create agents directories
GIVEN `slipway init --tools all --refresh` generates adapters
WHEN generated paths are inspected
THEN `.claude/agents`, `.codex/agents`, `.cursor/agents`, `.gemini/agents`, and `.opencode/agents` do not exist.

### Requirement: Update tests for the removed surface
REQ-007: Tests MUST reflect the removal of agent mapping config and hidden agent templates while preserving skill-surface contracts.

#### Scenario: Focused regression tests pass
GIVEN the implementation is complete
WHEN focused config, registry, health, template, bootstrap, and toolgen tests run
THEN removed agent-surface assumptions are gone and skill-surface contracts still pass.
