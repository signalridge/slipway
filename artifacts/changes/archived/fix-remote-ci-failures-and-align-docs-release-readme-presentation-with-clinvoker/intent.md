# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go, GitHub Actions, MkDocs, GoReleaser, Release Please
- Languages: Go, YAML, Markdown
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Conventions:

## Summary
fix remote CI failures and align docs release/readme presentation with clinvoker
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Fix the remote `CI` Windows checkout failure caused by long archived governance paths.
- Fix the remote `Docs` workflow so documentation builds continue to validate, while Pages deployment is skipped cleanly when GitHub Pages is not enabled.
- Align README presentation with the `clinvoker` style by adding project header imagery, CI/docs/release/report badges, and installation-channel badges.
- Align release-please PR copy with the `clinvoker` release flow guidance.
- Prevent future generated change slugs from creating extremely long artifact and worktree paths.

## Out of Scope
- Do not enable GitHub Pages repository settings unless the operator explicitly requests a remote settings change.
- Do not redesign the release workflow or package matrix beyond release/readme presentation hardening.
- Do not remove or rewrite historical archived governed artifacts.

## Constraints
- Preserve existing release artifact names and documented package sources.
- Keep changes compatible with existing GitHub Actions jobs and repository-local validation commands.
- Treat `clinvoker` as a reference pattern, not a file-for-file template.

## Acceptance Signals
- `actionlint` accepts the modified workflows.
- `mkdocs build --strict` succeeds.
- `goreleaser check` succeeds.
- `jq empty release-please-config.json .release-please-manifest.json` succeeds.
- Targeted slug-generation tests pass.
- `go build ./...` and `go test -timeout=20m ./... -count=1` pass.

## Open Questions
(none)

## Deferred Ideas
- Add a repository health check that warns when tracked file paths approach Windows default path limits.
- Add a future governed migration to shorten historical archive slugs if Windows clone compatibility becomes a product requirement.
- Enable GitHub Pages in repository settings later if docs publishing should go live; this change only makes docs validation and conditional deployment robust.

## Approved Summary
Confirmed by operator request on 2026-05-27: fix the remote CI failures after merging the compatibility-layer cleanup, and improve Slipway's release/docs/README presentation by using `/Users/yixianlu/Projects/clinvoker` as a reference for release documentation and badge/header presentation. Scope includes GitHub Actions hardening, README/release-please copy improvements, slug-length prevention, and full local plus remote validation. Scope excludes changing repository Pages settings, redesigning the release matrix, or rewriting historical archived governance artifacts.
