# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI
- Languages: Go
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Conventions:

## Summary
resolve workflow feedback from archived clinvoker end-to-end run
## Complexity Assessment
complex
Rationale: this change touches governed CLI workflow behavior, JSON/action contracts,
archival metadata, generated skill guidance, and end-to-end lifecycle verification.

## Guardrail Domains
external_api_contracts

## In Scope
- Resolve every non-deferred issue in
  `artifacts/changes/archived/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/workflow-feedback.md`.
- Reproduce or otherwise verify each referenced workflow-feedback item against the
  current implementation before claiming completion.
- Fix runtime, template, documentation, and tests needed for the workflow to make
  scaffold-only codebase maps and archived-feedback remediation relationships
  unambiguous.
- Exercise the full governed Slipway workflow for this remediation change,
  including evidence files and final verification.
- Record newly discovered workflow problems in feedback immediately if they are
  found during this governed run.

## Out of Scope
- Broad architecture rewrites unrelated to the referenced workflow feedback.
- Forcing previously documented deferred lifecycle-design items into this change
  unless they are necessary to resolve a current non-deferred feedback item.
- Changing external API behavior unrelated to Slipway's governed workflow,
  codebase-map, or archival/remediation surfaces.

## Constraints
- Preserve Slipway's thin-runtime / thick-host boundary: `change.yaml` remains
  current-state authority and lifecycle events remain audit evidence.
- Keep JSON/default handoff surfaces compact unless diagnostics are explicitly
  requested.
- Use repo-native Go tests and build commands.
- Newly created discovery changes must bind their default repo-local worktree
  before governed bundle artifacts are scaffolded, when the repository has a
  usable Git HEAD.
- Codebase-map scaffolding alone is not sufficient context; missing or
  scaffold-only maps must become deterministic baseline repository facts or stay
  visibly incomplete.
- Archived feedback remediation must persist an explicit source archive
  relationship so the final archive is not confused with the original archived
  bundle.

## Resolved Intake Findings
- `slipway codebase-map` ownership lives in `internal/engine/artifact` and is
  now the right place to populate deterministic baseline facts for missing or
  scaffold-only docs.
- Remediation archive relationships belong in durable `change.yaml` metadata
  and in `done --json` output, because final archive creation is the first point
  where the remediation bundle becomes terminal evidence.
- Early worktree binding is required for this feedback batch because the
  archived run showed root-scoped planning artifacts becoming misleading before
  S2 worktree preflight relocated them.
- Stale-evidence routing needs a bounded runtime distinction between planning
  drift, execution drift, and assurance-only verification edits.
- The root `slipway` catalog should stay a dispatch/metadata surface; full
  procedure text remains authoritative in the dedicated host skill or source
  skill template.

## Acceptance Signals
- Focused regression tests cover every code or contract fix.
- `go test -timeout=20m ./... -count=1` passes.
- `go build ./...` passes.
- The governed change reaches done-ready/done through Slipway commands with
  required verification evidence present.
- `workflow-feedback.md` has no unresolved non-deferred items without an explicit
  disposition and evidence pointer.

## Open Questions

## Deferred Ideas
- None for the currently actionable archived-feedback items.

## Approved Summary
Confirmed by the active continuation objective on 2026-05-25T10:59:44Z: complete
a governed remediation change for the archived clinvoker end-to-end workflow
feedback file. The change will fix all currently actionable feedback items,
test the full Slipway workflow, record any newly discovered workflow issues as
feedback, and verify the implementation with focused tests, full Go tests, build
checks, and Slipway lifecycle evidence. Unrelated architecture rewrites and
previously deferred long-term lifecycle redesigns are excluded unless they become
necessary to satisfy an actionable feedback item.
