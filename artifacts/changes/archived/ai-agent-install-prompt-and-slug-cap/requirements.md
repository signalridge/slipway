# Requirements
## Project Context
- Tech Stack: Go
- Conventions: mkdocs-based site under `docs/`; preserve `README.md:166` anchor link to `docs/installation.md#ai-tool-installation-prompt`; `MaxSlugLength` lives at `internal/model/identity.go:10`.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Requirements

### Requirement: Replace the `## AI Tool Installation Prompt` body in `docs/installation.md` with a short pointer prompt plus readable canonical prose.
REQ-001: The system MUST replace the prompt code block in the `## AI Tool Installation Prompt` section of `docs/installation.md` with a short (~10-line) agent-agnostic pointer prompt that instructs the agent to fetch `https://signalridge.github.io/slipway/installation/` and follow it. Below the prompt, readable prose MUST describe the Discovery / Install / Initialize / Verify / Report steps the agent should perform after fetching, so the canonical page is self-contained.

#### Scenario: Pointer prompt and canonical prose render cleanly
GIVEN the worktree contains the rewritten `docs/installation.md`
WHEN `mkdocs build --strict` runs against the worktree
THEN the build succeeds with no warnings treated as errors, the prompt code block renders as a short, copyable text block, and the Discovery/Install/Initialize/Verify/Report prose is visible directly below.

### Requirement: Mirror the short pointer prompt as a copyable preview block in `README.md`.
REQ-002: The system MUST add a fenced copyable preview of the short pointer prompt to `README.md` near `## Install` / `## Quick Install`, introduced by a short framing line asking the user to review the prompt before pasting and supervise the agent. The README MUST NOT duplicate the Discovery/Install/Initialize/Verify/Report prose — canonical detail lives in `docs/installation.md`.

#### Scenario: README preview is short and self-contained
GIVEN the worktree contains the updated `README.md`
WHEN a reader opens `README.md`
THEN a fenced code block of roughly 10 lines appears near `## Install` / `## Quick Install`, contains the same pointer prompt as `docs/installation.md`, and is selectable for copy-paste in one action.

### Requirement: Preserve the existing manual installation checklist in `docs/installation.md` unchanged.
REQ-003: The system MUST leave the manual installation checklist sections of `docs/installation.md` (everything outside the rewritten `## AI Tool Installation Prompt` section) byte-identical to the pre-change baseline on this branch.

#### Scenario: Non-prompt sections are unchanged
GIVEN the worktree's updated `docs/installation.md`
WHEN `git diff` is computed against the pre-change baseline for the file
THEN the only hunks touching content live inside the `## AI Tool Installation Prompt` section.

### Requirement: Preserve the `README.md:166` link to `docs/installation.md#ai-tool-installation-prompt`.
REQ-004: The system MUST keep the existing anchor link from `README.md` (currently line 166) pointing to a resolvable `docs/installation.md#ai-tool-installation-prompt` target by preserving the heading text `## AI Tool Installation Prompt` so mkdocs generates the same slug.

#### Scenario: README anchor link still resolves
GIVEN the worktree contains the updated `docs/installation.md` and `README.md`
WHEN `mkdocs build --strict` runs and the rendered docs site is inspected
THEN the anchor link from `README.md:166` resolves without a missing-anchor warning.

### Requirement: Reduce `MaxSlugLength` from 96 to 60 in `internal/model/identity.go` and keep the test green.
REQ-005: The system MUST change `MaxSlugLength` in `internal/model/identity.go` from `96` to `60`, and the existing `TestSlugifyTitleLimitsLongSlugs` test in `internal/model/identity_test.go` MUST continue to pass under the new value. If the test references the old number directly, the reference MUST be updated. The change MUST NOT affect existing slugs already on disk.

#### Scenario: SlugifyTitle caps at 60 for long inputs and tests stay green
GIVEN the new value `MaxSlugLength = 60`
WHEN `go test -count=1 ./internal/model/...` runs
THEN `TestSlugifyTitleLimitsLongSlugs` passes and asserts that a long-title slug is no longer than 60 characters.

AND GIVEN existing change directories under `artifacts/changes/` with slugs longer than 60 characters
WHEN `slipway status --json` reads those changes
THEN the existing slugs continue to load without error (the cap only affects newly created slugs).
