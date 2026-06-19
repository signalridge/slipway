# Requirements

## Requirements

### Requirement: P1 adapter selection
REQ-001: The system MUST support the P1 adapter IDs `pi`, `qwen`, `kiro`,
`copilot`, `windsurf`, and `kilo` through `slipway init --tools`, and
`--tools all` MUST include those IDs with the existing adapters in deterministic
sorted order.

#### Scenario: explicit P1 tool selection
GIVEN a workspace without generated adapter files
WHEN `slipway init --tools pi,qwen,kiro,copilot,windsurf,kilo` is run
THEN Slipway generates adapter surfaces for exactly those tool IDs without
rejecting them as unsupported.

#### Scenario: all tools selection
GIVEN the adapter registry contains the existing adapters plus the P1 adapters
WHEN `slipway init --tools all` is run
THEN the generated adapter set includes every registered adapter once, sorted
deterministically by tool ID.

### Requirement: P1 generated surfaces
REQ-002: The system MUST generate host-appropriate, project-local adapter
surfaces for each P1 tool while preserving the thin-adapter rule that generated
files route to the `slipway` CLI rather than implementing governance inside the
host.

#### Scenario: Pi prompts, skills, and settings
GIVEN a temporary workspace
WHEN `slipway init --tools pi` is run
THEN Slipway writes `.pi/prompts/slipway-*.md`,
`.pi/skills/slipway-*/SKILL.md`, `.pi/settings.json`, and the `.pi/slipway`
sentinel/ownership files.

#### Scenario: Copilot prompts and skills
GIVEN a temporary workspace
WHEN `slipway init --tools copilot` is run
THEN Slipway writes `.github/prompts/slipway-*.prompt.md`,
`.github/skills/slipway-*/SKILL.md`, and a dedicated Copilot sentinel/ownership
surface without requiring hook launchers.

#### Scenario: skill-first and workflow-style tools
GIVEN a temporary workspace
WHEN `slipway init --tools qwen,kiro,windsurf,kilo` is run
THEN Qwen and Kiro expose generated command skills, and Windsurf/Kilo expose
workflow-style command files plus generated skills at the documented paths.

### Requirement: adapter ownership and refresh safety
REQ-003: The system MUST keep generated P1 adapter files under Slipway
sentinel/ownership control, MUST auto-detect only previously generated adapters,
and MUST preserve user-owned files in adjacent host directories.

#### Scenario: bare host directories are not owned
GIVEN a workspace containing bare `.pi`, `.qwen`, `.kiro`, `.github`,
`.windsurf`, or `.kilocode` directories without Slipway sentinels
WHEN `slipway init --refresh` is run without explicit `--tools`
THEN Slipway does not treat those bare directories as generated adapters.

#### Scenario: refresh refuses modified generated files
GIVEN a workspace with generated P1 adapter files and a user-modified generated
file recorded in the ownership manifest
WHEN `slipway init --tools <tool> --refresh` is run
THEN Slipway refuses to overwrite the modified generated file and preserves its
contents.

#### Scenario: shared Copilot directory is preserved
GIVEN a workspace with user-owned files under `.github`
WHEN `slipway init --tools copilot --refresh` is run
THEN Slipway only updates Slipway-owned Copilot adapter files and preserves
unowned `.github` files.

### Requirement: documentation and manifest visibility
REQ-004: The system MUST document the P1 adapter IDs, generated paths,
invocation styles, settings behavior, refresh behavior, and surface manifest
entries.

#### Scenario: docs name P1 adapters
GIVEN the implementation is complete
WHEN a user reads the AI tool adapter reference docs
THEN `pi`, `qwen`, `kiro`, `copilot`, `windsurf`, and `kilo` are listed with
their generated paths and invocation styles.

#### Scenario: surface manifest is current
GIVEN adapter registry entries have changed
WHEN the surface manifest check is run
THEN `docs/SURFACE-MANIFEST.json` matches the live manifest derived from
Slipway authorities.

### Requirement: verification
REQ-005: The system MUST include automated verification for P1 adapter
generation, registry contracts, ownership/refresh behavior, docs tokens, and the
committed surface manifest.

#### Scenario: targeted toolgen proof
GIVEN the P1 adapter implementation is complete
WHEN `go test ./internal/toolgen/...` is run
THEN the tests pass.

#### Scenario: full repository proof
GIVEN targeted toolgen proof passes
WHEN `go test ./...` is run
THEN the repository test suite passes or any unrelated harness failure is
reported with concrete evidence.
