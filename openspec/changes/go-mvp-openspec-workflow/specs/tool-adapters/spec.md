## ADDED Requirements

### Requirement: Four Tool Targets
Tool adapter generation SHALL support four targets:
- `claude`
- `cursor`
- `codex`
- `opencode`

Each target SHALL define:
- `commands_dir`
- trigger prefix/style
- auto-detect paths

#### Scenario: Tool registry completeness
- **WHEN** adapter registry is loaded
- **THEN** it SHALL contain exactly four targets

### Requirement: CLI Canonical Name
Runtime CLI canonical command SHALL be `speclane`.

AI-tool command aliases are adapter-layer triggers only and SHALL route to the canonical CLI command.

#### Scenario: Adapter trigger routes to canonical CLI
- **WHEN** any generated AI-tool trigger is invoked
- **THEN** wrapper execution SHALL invoke `speclane <command>` semantics

### Requirement: Core Workflow Independence
Core workflow SHALL NOT depend on generated tool artifacts.

- lifecycle commands MUST run without generated adapter files
- `speclane init --tools none` SHALL be valid
- adapter generation failure SHALL not corrupt runtime state

#### Scenario: Core-only bootstrap
- **WHEN** repo is initialized with `--tools none`
- **THEN** `new/do/status/context/done/cancel/pivot/repair/analyze/review` SHALL still function

### Requirement: Trigger Mapping
Command trigger short-name family SHALL be `spl` across AI tools.
Exact trigger syntax SHALL remain adapter-specific.
Each tool SHALL use a distinct short-name structure:
- claude: `/spl:<command>`
- cursor: `/spl.<command>`
- codex: `/prompts:spl-<command>`
- opencode: `/spl-<command>`

Command semantics SHALL be identical across tools.

#### Scenario: Codex trigger example
- **WHEN** command `new` is generated for codex
- **THEN** trigger SHALL be `/prompts:spl-new`

### Requirement: Command Wrapper Generation
Adapter generation SHALL create one command wrapper file per CLI command per target.

Command set:
- `init`, `new`, `do`, `status`, `context`, `done`, `cancel`, `pivot`, `repair`, `analyze`, `review`

Wrapper files SHALL:
- stay concise (< 50 lines)
- describe invocation syntax
- route to CLI command
- avoid embedding governance logic

#### Scenario: Wrapper contains no gate logic
- **WHEN** generated wrapper file is inspected
- **THEN** it SHALL not define gate rules or policy tables

### Requirement: Optional Helper Guides (Non-Blocking)
Optional helper guides SHALL remain advisory and SHALL NOT become runtime gates.

Adapters MAY generate optional helper guides for:
- wave execution discipline
- command-check interpretation
- review/verification checklist usage

These guides are advisory only and SHALL NOT define runtime pass/fail.

#### Scenario: Missing helper guide
- **WHEN** optional helper guide is absent
- **THEN** runtime behavior SHALL remain unchanged

### Requirement: Deterministic Generation
Given same tool config and command set, generated adapter artifacts SHALL be byte-identical.

No timestamps/random IDs are allowed in generated content.

#### Scenario: Repeat generation stability
- **WHEN** generation is run twice with same inputs
- **THEN** produced files SHALL be byte-identical

### Requirement: Init Tools Flag
`speclane init --tools <list>` SHALL control adapter generation.

Supported values:
- explicit list (`claude,cursor` etc.)
- `all`
- `none`

`speclane init --refresh` SHALL regenerate selected adapter files deterministically.

#### Scenario: Multi-tool generation
- **WHEN** `speclane init --tools claude,cursor` runs
- **THEN** wrapper files SHALL be generated for both targets
