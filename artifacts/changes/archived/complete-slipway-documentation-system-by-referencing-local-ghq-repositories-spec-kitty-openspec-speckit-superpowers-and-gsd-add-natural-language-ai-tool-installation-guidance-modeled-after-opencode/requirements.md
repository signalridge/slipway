# Requirements

## Project Context
- Tech Stack: Go CLI, Markdown documentation, MkDocs Material
- Conventions: keep docs aligned with current tracked code; keep generated adapter contracts sourced from `internal/toolgen`; preserve `change.yaml` as lifecycle authority
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Requirements

### Requirement: Tracked documentation system
REQ-001: The repository MUST contain a tracked `docs/` documentation system and `mkdocs.yml` MUST point only at Markdown pages that exist in the current checkout.

#### Scenario: MkDocs navigation is real
GIVEN the repository is checked out cleanly
WHEN `mkdocs.yml` is read
THEN every nav target points to a tracked page under `docs/` and no placeholder-only nav entries remain.

#### Scenario: Reader can start from the docs home page
GIVEN a user opens the generated documentation
WHEN they read the docs home page
THEN they can navigate to installation, workflow, commands, AI-tool adapters, operator guidance, and contributor guidance.

### Requirement: Reference-informed documentation shape
REQ-002: The documentation MUST explicitly reflect patterns borrowed from local `ghq` references: spec-kitty, OpenSpec, Spec Kit, Superpowers, GSD, and OpenCode.

#### Scenario: Borrowed patterns are traceable
GIVEN the governed research artifact is inspected
WHEN the references and findings are reviewed
THEN each named reference repository has a concrete borrowed pattern recorded, not just a name-only citation.

#### Scenario: Borrowed patterns fit Slipway
GIVEN Slipway docs are read
WHEN reference-inspired sections appear
THEN they describe Slipway's actual commands, adapter paths, and lifecycle rather than copying reference-project behavior.

### Requirement: Complete user documentation path
REQ-003: The docs MUST explain what Slipway is, why it exists, how to install it, how to initialize a repository, how the governed lifecycle works, what commands exist, how AI-tool adapters are generated, and how contributors/operators verify or recover a local workspace.

#### Scenario: New user path
GIVEN a new user wants to adopt Slipway in a repository
WHEN they follow the docs from overview to installation and workflow
THEN they can initialize Slipway, choose supported AI-tool adapters, create a governed change, and understand the next/run/status/done loop.

#### Scenario: Operator path
GIVEN an operator is maintaining or debugging a Slipway workspace
WHEN they read the operator guidance
THEN they can identify authoritative state files, read health/status output, refresh adapters, and select verification commands.

### Requirement: AI-tool natural-language installation guidance
REQ-004: The installation docs MUST include copy-paste natural-language instructions that an AI coding tool can follow to install or build Slipway, initialize the current repository, select supported adapters including OpenCode, and verify the result.

#### Scenario: AI tool install prompt
GIVEN a user is inside an AI coding tool
WHEN they paste the documented installation prompt
THEN the prompt instructs the tool to inspect the current repo, avoid overwriting unrelated files, install or build Slipway by a documented path, run `slipway init --tools ...`, and verify generated surfaces.

#### Scenario: OpenCode path is explicit
GIVEN the user chooses OpenCode
WHEN the AI-tool installation guidance is followed
THEN the docs name `.opencode/skills`, `.opencode/commands/slipway-*.md`, and `/slipway-*` command usage.

### Requirement: Platform and package installation coverage
REQ-006: The installation docs MUST cover the package and artifact surfaces configured by the repository release pipeline for macOS, Linux, Windows, Go module installs, Nix, containers, and source builds, while distinguishing optional package-manager channels from guaranteed source/archive paths.

#### Scenario: Platform matrix is explicit
GIVEN a user reads the installation page
WHEN they look for their operating system and architecture
THEN they can identify the documented path for macOS amd64/arm64, Linux amd64/arm64, Windows amd64/arm64, or a cross-platform Go/source path.

#### Scenario: Optional package channels are not overstated
GIVEN package-manager publication depends on release workflow credentials
WHEN Homebrew, Scoop, AUR, Linux packages, or container instructions appear
THEN the docs describe them as release/package channels and provide fallback paths when a channel is unavailable.

### Requirement: README product entrypoint and visual flow
REQ-007: The README MUST be redesigned as a complete but non-redundant product entrypoint that introduces Slipway's purpose, design philosophy, core capabilities, install options, lifecycle flow, AI adapters, runtime files, and docs map, including a Mermaid lifecycle diagram.

#### Scenario: README explains product value before commands
GIVEN a new reader opens the repository
WHEN they read the README
THEN they see why Slipway exists, the design philosophy, core capabilities, and the governed lifecycle before deep reference links.

#### Scenario: README does not duplicate full docs
GIVEN a reader needs platform-specific installation or detailed operations
WHEN they use the README
THEN it gives a concise summary and links to the detailed docs instead of duplicating every package command or operator procedure.

### Requirement: Resolved Open Questions do not block intake
REQ-005: Intake and traceability checks MUST treat `(none)` and checked `## Open Questions` items as resolved, while unchecked checklist items and plain bullet questions remain blocking.

#### Scenario: Resolved questions advance
GIVEN `intent.md` has `## Open Questions` containing `(none)` or `- [x] ...`
WHEN intake progression evaluates the artifact
THEN the change can advance out of clarification/research when other intake evidence is valid.

#### Scenario: Unresolved questions still block
GIVEN `intent.md` has `## Open Questions` containing `- [ ] ...` or a plain bullet question
WHEN intake progression evaluates the artifact
THEN the change remains routed to research with `open_questions_detected`.
