# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI, generated Agent Skills templates
- Languages: Go, Markdown, Shell, Python
- Test Command: go test ./internal/toolgen ./cmd ./internal/engine/capability
- Build Command: go build ./...
- Conventions:

## Summary
optimize Slipway skill surfaces and quality guardrails without changing runtime behavior
## Complexity Assessment
complex
Rationale: the change is non-runtime but crosses generated skill templates,
generated reference documentation, and toolgen tests. It needs discovery to
separate high-value skill-quality improvements from marketplace-style expansion.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Improve generated skill-surface discoverability for exported host/support skills
  and public focus aliases without changing command semantics.
- Add lightweight skill-quality guardrails in tests for generated skill indexes,
  long reference usability, and script safety.
- Tighten high-traffic skill prose where it prevents misuse: explicit "when not
  to use" boundaries, entry/exit criteria, and evidence-focused review cues.
- Preserve the existing thin-runtime / thick-host model and generated adapter
  surfaces.

## Out of Scope
- No new workflow states, runtime gates, or state-machine behavior.
- No new public command modes beyond existing `--focus` / `--list-focuses`
  surfaces.
- No broad import of external/community skill catalogs into Slipway.
- No marketplace, installer, remote registry, or platform-specific prompt layer.

## Constraints
- Keep the implementation small and auditable.
- Do not weaken the default JSON handoff contract or diagnostics boundary.
- Keep generated `.codex` / `.claude` skill surfaces consistent through
  `internal/toolgen`.
- Prefer mechanical tests over new process layers.

## Acceptance Signals
- Generated workflow skill references enumerate exported skills and focus aliases
  accurately.
- Long skill references have a lightweight top-level navigation cue where needed.
- Toolgen tests prevent drift in skill index coverage, reference usability, and
  script safety.
- Targeted Go tests pass, including `go test ./internal/toolgen`.
- Broader relevant verification passes without changing Slipway runtime behavior.

## Open Questions

## Deferred Ideas
- A dedicated public `skill-quality-audit` skill.
- Remote skill registry, marketplace integration, or installer workflow.
- Adapter-specific `allowed-tools` rendering for every skill.

## Approved Summary
Approved 2026-05-25T14:56:56Z from the user's instruction to complete the
optimization through Slipway while avoiding over-engineering: implement a
small, non-runtime behavior change that improves Slipway skill discoverability,
reference usability, and skill-quality guardrails. Runtime governance semantics,
state transitions, command contracts, and broad skill-catalog expansion remain
out of scope.
