# Requirements

## Requirements

### Requirement: Help Placeholders Match CLI Contracts
REQ-001: The system MUST render CLI help placeholders for hydration reference
flags with a meaningful value name that matches the generated documentation
contract.

#### Scenario: Hydrate ref help is readable
GIVEN the current Slipway source checkout
WHEN an operator runs `go run . status --help`, `go run . review --help`, or
`go run . health --help`
THEN the `--hydrate-ref` flag help shows `<skill-id>/<name>` as the argument
placeholder and does not render `--hydrate-ref --hydrate`.

### Requirement: Generated Skill Descriptions Match Routing Logic
REQ-002: Generated skill descriptions and source frontmatter MUST describe the
current lifecycle routing logic for S3 review peers, worktree preflight,
security review selection, git recovery support, wave orchestration, and
CLI-only helper tools.

#### Scenario: Skill descriptions do not claim stale routing
GIVEN generated skill source templates under `internal/tmpl/templates/skills`
WHEN a host or agent reads the exported description/frontmatter
THEN the description matches the current Go routing authorities and does not
claim unsupported path-glob, invalid-worktree, post-review, or sequential-only
behavior.

### Requirement: Docs And Diagrams Match Runtime Storage And Surfaces
REQ-003: User-facing docs and diagram descriptions MUST distinguish governed
bundle files from git-local runtime evidence, generated workflow command
surfaces from CLI-only helper namespaces, and CLI-owned resolution from
host-owned subagent dispatch.

#### Scenario: Operator docs point to the right evidence location
GIVEN a governed change with runtime task or wave evidence
WHEN an operator follows docs in `docs/index.md`, `docs/operator-guide.md`, or
`docs/installation.md`
THEN the docs direct them to git-local Slipway runtime evidence paths for task
and wave evidence, while keeping bundle-local `events/` and `verification/`
under `artifacts/changes/<slug>/`.

#### Scenario: AI tool docs do not overstate helper surfaces
GIVEN `slipway tool` is a public CLI-only helper namespace
WHEN an agent reads `docs/ai-tools.md` or the workflow skill
THEN the docs say generated host command surfaces cover generated workflow
commands and that helper subcommands are invoked directly through
`slipway tool ...`.

### Requirement: Surface Inventory And Tests Stay Fresh
REQ-004: The committed surface inventory and focused tests MUST be refreshed or
kept passing after docs, help, or generated skill source changes.

#### Scenario: Surface checks pass after alignment
GIVEN the aligned docs/templates/help implementation
WHEN the maintainer runs targeted command/template/toolgen tests and the surface
manifest check
THEN the checks pass without stale generated-surface or documentation-token
failures.
