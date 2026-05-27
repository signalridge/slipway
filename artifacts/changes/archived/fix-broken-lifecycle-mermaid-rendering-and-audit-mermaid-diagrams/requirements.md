# Requirements

## Project Context
- Tech Stack: Go CLI, Markdown documentation
- Conventions: Keep repository docs concise, use focused Go tests for behavior changes, and preserve Slipway lifecycle authority semantics.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Markdown, Go

## Requirements

### Requirement: README Lifecycle Mermaid renders reliably
REQ-001: `README.md` MUST contain a GitHub-compatible `## Lifecycle` Mermaid diagram that avoids the prior rendering failure.

#### Scenario: README lifecycle diagram parses
GIVEN the Mermaid block under `README.md` `## Lifecycle`
WHEN it is parsed with Mermaid CLI
THEN parsing succeeds without syntax errors.

### Requirement: Mermaid diagram audit stays bounded
REQ-002: Mermaid blocks in `README.md` and `docs/*.md` MUST be checked for syntax failures, with only low-risk compatibility/readability edits applied.

#### Scenario: Repository docs Mermaid blocks parse
GIVEN every `mermaid` fenced code block in `README.md` and `docs/*.md`
WHEN each block is parsed with Mermaid CLI
THEN all blocks parse successfully.

### Requirement: README ends with the requested repo status image
REQ-003: `README.md` MUST end with a repo status image embed using the user-provided `camo.githubusercontent.com` URL.

#### Scenario: README status image is appended
GIVEN the final section of `README.md`
WHEN the Markdown is inspected
THEN it includes a `Repository Status` section whose image source is the provided camo URL.

### Requirement: Explicit none markers do not block Open Questions
REQ-004: Open Questions detection MUST treat explicit none markers such as `- None.` as resolved while preserving blockers for unchecked checklist items and substantive unresolved prose.

#### Scenario: Bullet none advances intake
GIVEN an `intent.md` Open Questions section containing `- None.`
WHEN S0 intake evaluates open questions
THEN it treats the section as resolved.

#### Scenario: Unchecked checklist still blocks
GIVEN an `intent.md` Open Questions section containing `- [ ] None.`
WHEN S0 intake evaluates open questions
THEN it treats the section as unresolved.

### Requirement: Open Questions behavior is documented and tested
REQ-005: The accepted resolved-marker behavior MUST be covered by focused tests and documented in the workflow guide.

#### Scenario: Focused coverage explains the behavior
GIVEN the Open Questions helper, intake progression, and workflow docs
WHEN the change is inspected or tested
THEN `- None.` is covered as resolved and unchecked checklist items remain covered as blockers.
