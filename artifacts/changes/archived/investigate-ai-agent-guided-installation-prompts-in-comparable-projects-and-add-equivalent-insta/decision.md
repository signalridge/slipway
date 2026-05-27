# Decision
## Project Context
- Tech Stack: Go, MkDocs
- Conventions: Documentation uses concise sections, copyable fenced commands, stable relative links, and `mkdocs build --strict` as the rendering proof.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Alternatives Considered

### Approach 1: Keep existing installation documentation unchanged
- Description: Treat the existing `docs/installation.md#ai-tool-installation-prompt` as sufficient.
- Tradeoffs: Lowest churn, but the AI-agent prompt stays buried below the platform install matrix and does not match the user-requested first-contact pattern.

### Approach 2: Add a short README entry and a prominent short installation-page prompt
- Description: Add a compact "Install With An AI Agent" entry to README and `docs/installation.md`, while preserving the existing detailed prompt as the canonical checklist.
- Tradeoffs: Adds small duplication, but keeps the detailed installation matrix in one canonical page and gives users an immediate copy/paste path.

### Approach 3: Add a dedicated AI-agent installation page
- Description: Create a new page such as `docs/agent-install.md` and add it to navigation.
- Tradeoffs: Clean separation, but creates another page to keep synchronized for a small documentation feature.

## Selected Approach
Use Approach 2.

The implementation will add a compact AI-agent install prompt to `README.md` and a more prominent short prompt near the top of `docs/installation.md`. The existing detailed "AI Tool Installation Prompt" remains the canonical checklist for OS/architecture detection, release-source selection, adapter initialization, and verification.

## Interfaces and Data Flow
- No Go API, CLI command, JSON schema, state file, or generated adapter interface changes.
- Documentation flow changes only:
  - `README.md` points first-time users to a short AI-agent setup prompt and the canonical installation page.
  - `docs/installation.md` keeps manual installation paths and hosts both the short prompt and the detailed agent checklist.
  - Existing adapter path contracts remain aligned with `docs/ai-tools.md`.

## Rollout and Rollback
- Rollout: edit Markdown docs and governed artifacts in the feature worktree.
- Verification: run `mkdocs build --strict`; run `go run . validate --json` after governed evidence is updated.
- Rollback: revert the README and installation documentation edits. No migration, generated adapter refresh, package publishing, or release rollback is required.

## Risk
- Documentation drift: mitigated by linking the short prompt to the canonical detailed installation checklist instead of duplicating every platform command.
- Prompt-safety drift: mitigated by explicitly instructing agents to use project-owned release sources, avoid unverified same-name packages, preserve user-owned AI-tool files, and avoid unrelated promotional/social actions.
- Render/link breakage: mitigated by `mkdocs build --strict`.
