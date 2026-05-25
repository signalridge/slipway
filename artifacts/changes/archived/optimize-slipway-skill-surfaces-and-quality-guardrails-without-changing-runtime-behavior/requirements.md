# Requirements

## Project Context
- Tech Stack: Go CLI, generated Agent Skills templates
- Conventions: Go-owned capability metadata, deterministic toolgen output, generated surfaces tested through `internal/toolgen`.
- Test Command: `go test ./internal/toolgen ./cmd ./internal/engine/capability`
- Build Command: `go build ./...`
- Languages: Go, Markdown, Shell, Python

## Requirements

### Requirement: Generated lookup surfaces expose public focus aliases without changing runtime behavior.
REQ-001: The generated workflow skill index and command reference MUST make existing public `--focus` aliases discoverable from Go-owned `capability.surfacePolicy` metadata.

#### Scenario: Public focus aliases appear in generated references
GIVEN adapter skill surfaces are generated
WHEN an operator reads the workflow command reference or skill index
THEN the existing public focus aliases are listed with their command, alias, backing skill, and summary.

#### Scenario: Non-exported support skills remain non-host skills
GIVEN a focus alias is backed by a non-exported support skill
WHEN the workflow skill index is generated
THEN the alias is discoverable as a command selector but no direct host `SKILL.md` path is emitted for that non-exported skill.

### Requirement: Skill-quality guardrails stay mechanical and lightweight.
REQ-002: The repository MUST add focused tests that prevent drift in public focus rendering, skill-index lookup coverage, and long reference usability without adding new process or runtime layers.

#### Scenario: Public-focus drift is caught
GIVEN `capability.surfacePolicy` changes
WHEN `go test ./internal/toolgen ./internal/engine/capability` runs
THEN generated command references and skill-index tests fail if the public focus aliases are not rendered.

#### Scenario: Long references remain navigable
GIVEN a reference markdown file grows beyond the long-reference threshold
WHEN `go test ./internal/toolgen` runs
THEN the test fails unless the file includes a top-level quick navigation cue.

### Requirement: High-traffic skill/reference prose prevents misuse while staying concise.
REQ-003: Generated or template-owned skill prose MUST clarify lookup authority, when not to use broad routes, and top-level navigation for long references without duplicating full command semantics.

#### Scenario: Workflow skill preserves authority boundaries
GIVEN an operator reads the workflow skill
WHEN command/focus lookup guidance appears
THEN the prose states that CLI responses remain authoritative and lookup aids do not alter governed host selection.

#### Scenario: Long references offer immediate orientation
GIVEN a long reference file is loaded
WHEN the reader reaches the top of the file
THEN a `## Quick Navigation` section lists the main sections so the reader can jump to the relevant material without scanning the full file.

### Requirement: Existing thin-runtime / thick-host contract is preserved.
REQ-004: The change MUST NOT add lifecycle states, gates, command flags, JSON fields, exported host skills, retired catalog artifacts, or new runtime behavior.

#### Scenario: Existing generated-surface tests still pass
GIVEN the focused implementation is complete
WHEN existing toolgen and capability tests run
THEN exported host allowlists, command metadata, no-retired-catalog assertions, and generated script checks still pass.

#### Scenario: Full project verification remains green
GIVEN focused tests pass
WHEN `go test ./...` and `go build ./...` run
THEN the broader Slipway runtime remains unchanged and buildable.
