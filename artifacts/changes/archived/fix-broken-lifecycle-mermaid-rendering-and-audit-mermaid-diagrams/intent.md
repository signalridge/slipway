# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: 
- Languages: Markdown, Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: 

## Summary
Fix broken Lifecycle Mermaid rendering, audit the repository's Mermaid diagrams for renderability/readability improvements, add a repo status image at the end of `README.md`, and fix repeated Open Questions false positives for explicit none markers such as `- None.`.
## Complexity Assessment
simple
Rationale: bounded documentation and small Go normalization fix, with validation via Mermaid parsing and targeted repo-native tests.

## In Scope
- Fix the `## Lifecycle` Mermaid block in `README.md` so it renders correctly.
- Inspect Mermaid blocks in `README.md` and `docs/*.md` for syntax issues and clear low-risk improvements.
- Add a repo status image at the end of `README.md`, using the provided `camo.githubusercontent.com` image URL as the example/source.
- Update Open Questions detection so explicit none markers such as `- None.` do not keep intake stuck.
- Update focused tests and docs for the Open Questions resolved-marker behavior.

## Out of Scope
- No changes to Slipway JSON contracts, public command surfaces, or lifecycle state ordering.
- No broad lifecycle-state redesign, new workflow stages, or changes to unresolved checklist/prose question behavior.
- No broad prose rewrite unrelated to Mermaid rendering/readability, the README status image, or the Open Questions marker behavior.
- No generated adapter refresh unless the docs change explicitly requires it.

## Constraints
- Preserve existing terminology and command names.
- Prefer GitHub-compatible Mermaid syntax.
- Keep README additions concise and placed at the end as requested.
- Treat the provided `camo.githubusercontent.com` URL as an image embed, not as configuration or allowlist text.
- Preserve unresolved-question blocking for unchecked checklist items and substantive prose/bullets.

## Acceptance Signals
- Every Mermaid code block in the touched Markdown files parses successfully with Mermaid CLI or an equivalent local parser.
- `README.md` ends with a repo status image embed using the provided `camo.githubusercontent.com` image URL.
- Focused tests prove `- None.` is accepted as a resolved Open Questions marker while unchecked checklist entries still block.
- Markdown renders as valid fenced code blocks with no obvious broken table or heading structure.
- Repo-native validation (`go test ./...` and/or narrower docs validation if available) passes or any inability to run is recorded.

## Open Questions
(none)

## Approved Summary
Confirmed 2026-05-27T11:37:55Z and expanded 2026-05-27 after user follow-up: fix the README Lifecycle Mermaid rendering issue, audit Mermaid blocks in README/docs for low-risk renderability and readability improvements, append a repo status image to the end of README using the provided `camo.githubusercontent.com` URL, and fix repeated Open Questions false positives for explicit none markers such as `- None.`. Do not redesign lifecycle stages or weaken blocking for unchecked checklist items and substantive unresolved questions. Primary acceptance signals are Mermaid parse success for touched diagrams, valid Markdown structure, the repo status image at the README end, focused Open Questions tests, and repo-native validation where practical.
