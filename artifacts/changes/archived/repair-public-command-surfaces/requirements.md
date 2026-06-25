# Requirements

## Requirements

### Requirement: Registry-backed public command inventory
REQ-001: Slipway MUST represent every non-hidden root help command in the
command registry, unless the command is intentionally hidden or excluded by a
documented test exemption.

#### Scenario: config is visible in root help and manifest
GIVEN `slipway --help` lists `config` under setup commands
WHEN the surface manifest is built
THEN the manifest includes a `command/config` row and the command reference
contains `slipway config`.

### Requirement: CLI-only setup command semantics
REQ-002: The `config` command SHALL remain a CLI-only setup/config surface unless
explicitly changed by a future product decision.

#### Scenario: config does not generate host command skills
GIVEN `config` is in the command registry
WHEN command skill surfaces are generated
THEN no `$slipway-config` or equivalent host command wrapper is generated.

### Requirement: Stable JSON token consistency
REQ-003: Command documentation MUST use the canonical manifest JSON tokens for
stable command-contract examples in English, Japanese, and Chinese command
references.

#### Scenario: run and handoff tokens are stable
GIVEN detailed command pages list stable JSON manifest tokens
WHEN docs-token tests inspect those pages
THEN `run JSON` is documented as `slipway run --json` and `handoff JSON` is
documented as `slipway handoff show --json`.

### Requirement: Adapter inventory consistency
REQ-004: Adapter documentation and diagram copy MUST agree with the current
tool registry inventory.

#### Scenario: design prose matches supported tools
GIVEN the tool registry supports ten tool IDs
WHEN design and adapter documentation describe thin adapters
THEN the wording names or points to the full current ten-tool adapter inventory
instead of only Claude, Codex, Cursor, and OpenCode.

### Requirement: No unsupported public review flag
REQ-005: The `review` command MUST NOT expose a visible flag that always returns
an unsupported MVP error.

#### Scenario: artifact flag is removed
GIVEN a user runs `slipway review --help`
WHEN the help text is rendered
THEN `--artifact` is absent and no flag-contract exemption is needed for it.

### Requirement: Continuity handoff section usage
REQ-006: README-level handoff examples MUST show how to refresh a named handoff
section through `handoff write --section`.

#### Scenario: sectioned handoff is documented
GIVEN a user reads the README continuity commands
WHEN they need to refresh one section from stdin
THEN the README shows `slipway handoff write --section <name>`.

### Requirement: Low-risk misleading residue cleanup
REQ-007: Internal command names and residual tests SHALL be renamed where the
current name suggests retired public behavior or misstates the hidden command
purpose.

#### Scenario: hidden root command and retired command test names are clear
GIVEN maintainers inspect hidden root command helpers or retired command tests
WHEN they read file and function names
THEN names describe the current hidden root-path helper and retired command
guard rather than suggesting an active `learn` surface.
