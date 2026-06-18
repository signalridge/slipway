# Intent

## Summary
Align Slipway's documentation, generated skill descriptions, flowchart
descriptions, and CLI help surfaces with the current code contracts.

## Complexity Assessment
complex
Rationale: this is a cross-surface alignment change touching user-facing docs,
generated skill templates, command help, and docs diagrams. The implementation
must be driven by live CLI/code evidence rather than stale generated prose.

## Guardrail Domains
None.

## In Scope
- Audit docs, generated skill descriptions, and flowchart/diagram descriptions
  against current code and live CLI help.
- Fix confirmed drift in docs under `docs/`, generated skill templates under
  `internal/tmpl/templates/skills/`, command-surface partials, and CLI help text.
- Include verified help output defects, specifically the `--hydrate-ref`
  placeholder rendered by Cobra help.
- Keep `docs/SURFACE-MANIFEST.json` and related toolgen guardrails consistent
  when source changes require regeneration.

## Out of Scope
- No lifecycle or governance behavior changes beyond help/description alignment.
- No broad rewrite of command taxonomy or capability routing unless required by
  a confirmed mismatch.
- No edits to unrelated root checkout dirty files or inactive worktrees.

## Constraints
- Use the bound worktree
  `/Users/yixianlu/ghq/github.com/signalridge/slipway/.worktrees/align-docs-skills-help`
  as the authority for edits and verification.
- Preserve unrelated user changes, including pre-existing root checkout
  `artifacts/codebase/*` modifications.
- Treat `go run . --help`, subcommand help, `commandRegistry`,
  `ResolveNextSkill`, `SelectedReviewSkills`, and state path helpers as
  code authorities for claims.

## Acceptance Signals
- Live help no longer renders `--hydrate-ref --hydrate`; it renders a meaningful
  value placeholder aligned with docs.
- Generated skill/template descriptions no longer claim stale S3, worktree,
  security-review, command-surface, or wave-ordering behavior.
- Docs and diagram accessible text distinguish CLI-owned state changes,
  generated host command surfaces, CLI-only helper tools, and git-local runtime
  evidence paths.
- Targeted Go tests for command help/template contracts and toolgen surfaces
  pass; docs/manifest checks pass or are refreshed.
- `go run . validate --json` and `go run . next --json --diagnostics` show the
  governed change can progress through required lifecycle gates.

## Open Questions
None.

## Deferred Ideas
- A future guard could compare root help grouping with generated taxonomy if the
  project wants those classifications to be identical. This change documents the
  confirmed surface contract instead of redesigning taxonomy.

## Approved Summary
Confirmed from the user's explicit request on 2026-06-18T02:12:36Z: align all
docs skill descriptions, flowchart descriptions, and help-related surfaces with
latest code logic, using parallel subagents first to investigate errors and
gaps. Scope includes fixing confirmed docs/templates/help drift and excludes
unrelated lifecycle behavior changes.
