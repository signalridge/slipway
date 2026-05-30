# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go, Markdown
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
resolve issue 21: clarify JSON stdin classification contract in exported agent surfaces
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Update `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl` so the primary exported workflow skill states that explicit classification for `slipway new --json` is supplied through JSON stdin, not command-line flags.
- Add a minimal JSON stdin example for `slipway new --json` in at least one exported agent-facing surface.
- Update `internal/tmpl/templates/_partials/command-new-body.tmpl` so `/slipway-new` documents the explicit JSON stdin classification path for AI callers that already know the classification.
- Update generated command-reference metadata or rendering so the `slipway new` entry includes stdin classification notes in addition to CLI flags.
- Add or update template/toolgen tests proving generated Codex and Claude surfaces preserve the JSON stdin contract.

## Out of Scope
- Do not add `--guardrail-domain`, `--complexity`, or `--needs-discovery` CLI flags.
- Do not change `cmd/new.go` runtime classification behavior beyond what tests require for generated surfaces.
- Do not redesign the broader agent command-surface documentation system.

## Constraints
- Keep the change scoped to exported agent-facing contract clarity.
- Preserve the existing guidance that normal users should not manually choose internal routing labels when Slipway can infer them.
- Keep generated Codex and Claude surfaces aligned.

## Acceptance Signals
- Generated workflow skill explicitly says `guardrail_domain`, `complexity`, and `needs_discovery` are JSON stdin fields for `slipway new --json`, not CLI flags.
- `/slipway-new` mentions the explicit JSON stdin classification path without encouraging unsupported flags.
- Generated command reference includes JSON stdin classification shape notes for `slipway new`.
- Template/toolgen tests fail if the JSON stdin contract text or minimal example disappears.
- `go test ./...` and `go build ./...` pass.

## Open Questions
- [x] Verify the exact generated-surface test helpers before choosing whether to update snapshot expectations, direct string assertions, or both. Resolved in `research.md`: direct generated-surface string assertions in `internal/toolgen/toolgen_test.go` cover the relevant Codex and Claude outputs without adding snapshots.

## Deferred Ideas
- Future richer stdin schema metadata for every command that accepts structured stdin.

## Approved Summary
Approved by user on 2026-05-30T14:28:02Z. Resolve GitHub issue #21 by clarifying exported agent-facing contracts for `slipway new --json`: explicit classification fields are provided via JSON stdin, not unsupported CLI flags. The implementation will update the workflow skill template, `/slipway-new` prompt partial, command-reference generation, and regression tests. It will not add new CLI flags or change the runtime classification semantics.
