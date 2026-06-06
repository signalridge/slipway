# Requirements
## Project Context
- Tech Stack: Go
- Conventions: Slipway Agent Principles (CLAUDE.md). Source-then-regenerate.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Cobra --help text matches real behavior
REQ-001: Every command's `--help` MUST describe real behavior: each flag-usage
line names a flag actually registered on that command with the correct default,
and `Short`/`Long` prose contains no phantom flags or stale options.

#### Scenario: Help audit
GIVEN any `slipway <cmd> --help`
WHEN its flag-usage lines and Short/Long prose are compared to the command's
registered FlagSet and behavior
THEN no described flag is absent from the FlagSet, no default is wrong, and any
genuine behavior divergence is recorded as an out-of-scope note (logic unchanged).

### Requirement: Registry Arguments cover every flag
REQ-002: `commandArguments(id)` MUST list every non-hidden Cobra flag for each
command, and a new exported `CommandArguments(id)` MUST expose it for the guard.

#### Scenario: Registry coverage
GIVEN a command's registered non-hidden flags
WHEN `toolgen.CommandArguments(id)` is rendered
THEN every such flag (except documented exemptions) appears in the string.

### Requirement: Body templates list core action flags
REQ-003: Each `command-*-body.tmpl` that has a `## Flags` section MUST list its
core action flags (e.g. pivot `--reroute/--rescope`, done `--all-ready`, next
`--no-auto-pass`, review/validate `--focus/--list-focuses`).

#### Scenario: Body flags
GIVEN a command surface with a `## Flags` section
WHEN the surface is generated
THEN no core action flag of that command is missing from the section.

### Requirement: Generated skill surfaces cite real flags
REQ-004: Entry and `slipway-*` host skill surfaces MUST NOT cite a non-existent
flag and MUST NOT omit a real flag they reference.

#### Scenario: Skill citations
GIVEN a generated skill surface that cites `slipway <cmd> --<flag>`
WHEN it is checked against the FlagSet
THEN the flag exists (existing guard) and required flags are present (new guard).

### Requirement: References regenerate consistent
REQ-005: `references/command-reference.md` and `references/skill-index.md` MUST
regenerate consistent with the real flag set after the source fixes.

#### Scenario: Reference regen
GIVEN `slipway init --refresh`
WHEN the references are regenerated
THEN they list the real flags with zero drift from `--help`.

### Requirement: docs/ and README cite real flags
REQ-006: `docs/` and `README.md` command/flag usage MUST cite no non-existent
flag and no stale option.

#### Scenario: Docs audit
GIVEN a `slipway <cmd> --<flag>` reference in docs/README
WHEN compared to the FlagSet
THEN the flag exists and the surrounding usage is current.

### Requirement: Reverse flag-contract guard fails closed
REQ-007: A test MUST fail when any non-hidden, non-help Cobra flag is absent
from `CommandArguments(id)`, with a documented exemption list.

#### Scenario: Guard
GIVEN a flag registered on a command but missing from its Arguments string
WHEN the reverse guard test runs
THEN the test fails (CI fails closed), unless the flag is in the exemption list.

### Requirement: Entry skill is discoverable on task intent
REQ-008: The entry skill `description` MUST carry task-side trigger language (not
only insider lifecycle terms); the SessionStart hook output MUST reference
loading the slipway skill; the three-layer boundary MUST be stated.

#### Scenario: Discoverability
GIVEN an agent arriving with "change code / fix a bug" intent
WHEN it reads the entry skill description and the SessionStart hook output
THEN both surface task-side cues that point at the slipway skill, and the
entry vs `slipway:*` vs `slipway-*` boundary is explicit.
