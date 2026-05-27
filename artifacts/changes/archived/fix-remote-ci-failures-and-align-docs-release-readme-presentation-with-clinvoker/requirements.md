# Requirements

## Project Context
- Tech Stack: Go, GitHub Actions, MkDocs, GoReleaser, Release Please
- Conventions: local-first Slipway governance, repo-native validation, least-privilege CI permissions, deterministic release automation
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Markdown

## Requirements

### Requirement: Remote CI hardening

REQ-001: Slipway MUST keep the remote `CI` workflow green after the compatibility-layer cleanup, including the Windows checkout/test path.

Acceptance:
- Windows checkout config supports long tracked paths without requiring a mutable system Git config.
- Windows matrix tests run after checkout.
- Full CI run for the latest `main` commit succeeds.

### Requirement: Docs workflow robustness

REQ-002: Slipway MUST keep documentation builds validated while avoiding deployment failure when GitHub Pages is disabled.

Acceptance:
- `mkdocs build --strict` remains required.
- Upload/deploy steps run only when GitHub Pages is enabled.
- The docs workflow has enough permissions to inspect Pages status.
- Latest `Docs` workflow for `main` succeeds.

### Requirement: Clinvoker-inspired presentation

REQ-003: Slipway MUST improve README presentation by adopting clinvoker-style header imagery and badge visibility without changing project identity.

Acceptance:
- README includes a prominent project header image.
- README includes CI, Docs, Release, Go Report Card, and install-channel badges.
- Existing install and documentation links remain visible.

### Requirement: Release PR guidance

REQ-004: Slipway MUST make Release Please PR copy clearer and closer to the clinvoker release-documentation style.

Acceptance:
- `release-please-config.json` describes the review/merge/release flow.
- Release Please configuration remains valid JSON.
- Release automation surfaces continue to validate.

### Requirement: Future slug path safety

REQ-005: Slipway MUST prevent newly generated change slugs from recreating extreme worktree/artifact path lengths.

Acceptance:
- `SlugifyTitle` caps generated slug length.
- Collision suffix generation preserves the numeric suffix inside the cap.
- Focused unit tests cover truncation and collision suffix behavior.

### Requirement: Validation evidence

REQ-006: The change MUST be validated locally and against the latest remote GitHub Actions runs.

Acceptance:
- Local workflow/config/docs/release/build/test commands pass.
- Latest remote `main` runs for CI, Docs, Security, and Release Please succeed.
- Any older failed runs are identified as historical failures on superseded commits.

## Non-Functional Requirements
- Keep the implementation targeted and compatible with existing CI/release conventions.
- Avoid requiring repository settings changes as part of the code fix.
- Keep rollback simple through normal Git revert of the fix commits.
