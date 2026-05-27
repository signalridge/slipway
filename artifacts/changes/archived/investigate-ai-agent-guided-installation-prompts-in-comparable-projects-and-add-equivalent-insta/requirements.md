# Requirements
## Project Context
- Tech Stack: Go, MkDocs
- Conventions: Documentation uses concise sections, copyable fenced commands, stable relative links, and `mkdocs build --strict` as the rendering proof.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Requirements

### Requirement: Make AI-agent installation discoverable from the README.
REQ-001: The README MUST include a compact AI-agent installation entry that a user can copy into an AI coding tool without reading the full installation page first.

#### Scenario: README first-contact flow
GIVEN a user lands on the repository README
WHEN they look for installation guidance for an AI coding tool
THEN they can find a short copyable prompt and a link to the detailed installation instructions.

### Requirement: Keep `docs/installation.md` as the canonical detailed installation guide.
REQ-002: The installation documentation MUST retain the complete platform matrix and detailed AI-tool installation checklist while adding a prominent short agent prompt near the installation entry.

#### Scenario: Documentation canonical flow
GIVEN a user opens `docs/installation.md`
WHEN they choose between manual and AI-assisted setup
THEN the page presents the short AI-agent prompt, the detailed AI-tool prompt, and the existing release/package/source install paths without contradiction.

### Requirement: Preserve safety and current adapter contracts.
REQ-003: The AI-agent guidance MUST instruct agents to use project-owned release sources, avoid unverified same-name packages, preserve user-owned AI-tool files, and verify generated adapter paths that match current Slipway contracts.

#### Scenario: Safe agent setup flow
GIVEN an AI agent follows the Slipway install guidance
WHEN prerequisites, release channels, or tool adapters are ambiguous
THEN the agent stops or asks instead of inventing installers, overwriting user files, or performing unrelated promotional/social actions.

### Requirement: Prove the documentation remains renderable and aligned.
REQ-004: The change MUST pass the repository documentation build and governed validation after the documentation edits.

#### Scenario: Verification flow
GIVEN the documentation changes are complete
WHEN verification runs
THEN `mkdocs build --strict` passes and Slipway validation reports no blockers for the current governed state.
