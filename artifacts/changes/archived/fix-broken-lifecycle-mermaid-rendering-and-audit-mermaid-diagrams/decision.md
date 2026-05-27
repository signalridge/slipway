# Decision

## Project Context
- Tech Stack: Go CLI, Markdown documentation
- Conventions: Prefer small, deterministic helpers; keep documentation changes scoped; verify with focused tests before broader validation.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Markdown, Go

## Alternatives Considered

### A. Documentation-only workaround
Require artifacts to use exactly `(none)` for resolved Open Questions and only update docs/README.

Tradeoffs:
- Lowest code risk.
- Does not solve the user's repeated `- None.` false-positive problem.
- Keeps workflow friction in manually edited artifacts and existing bundles.

### B. Normalize explicit none markers in the shared helper
Update `HasBlockingOpenQuestions` to accept exact none markers after optional bullet punctuation, while keeping checked/unchecked checklist semantics explicit.

Tradeoffs:
- Small Go behavior change in a shared helper.
- Directly fixes repeated false positives for common authoring forms such as `- None.`.
- Requires focused tests to avoid hiding real open questions.

### C. Template-only update
Change generated artifact guidance to prefer `(none)` and avoid touching runtime behavior.

Tradeoffs:
- Helps future generated artifacts.
- Does not fix existing artifacts or operator-written `- None.` content.
- Leaves the root cause in the shared helper.

## Selected Approach
Use Approach B and keep the README/docs edits separate and bounded.

The selected implementation updates `internal/stringutil.HasBlockingOpenQuestions` to treat exact explicit none markers as resolved after stripping optional bullet punctuation. It keeps unchecked checklist items blocking, documents `- None.` in `docs/workflow.md`, replaces the README Lifecycle Mermaid block with a state diagram style already used by `docs/workflow.md`, and appends the requested repo status image to the README end.

## Interfaces and Data Flow
- No public CLI command, JSON field, or lifecycle state ordering changes.
- Intake and governance traceability continue to call `stringutil.HasBlockingOpenQuestions`.
- Data flow remains: `intent.md` -> `LastMarkdownSectionContent` -> `HasBlockingOpenQuestions` -> intake/governance readiness.
- README/docs Mermaid blocks remain plain Markdown fenced code blocks.

## Rollout and Rollback
- Rollout: commit the helper/test/docs/README changes together after focused tests, Mermaid parsing, and full repo validation.
- Rollback: revert this change. The previous strict behavior would return, requiring `(none)` or checked checklist entries to clear Open Questions.
- Verification commands:
  - `go test ./internal/stringutil ./internal/engine/progression -run 'TestHasBlockingOpenQuestions|TestAdvanceIntake_OpenQuestionsUseResolvedItemSemantics' -count=1`
  - Mermaid CLI parse over all `README.md` and `docs/*.md` Mermaid blocks.
  - `go test ./...`

## Risk
- Low risk of over-normalization because only exact none markers are accepted after stripping simple bullet punctuation.
- Low docs risk because Mermaid blocks are parsed locally and README image addition is isolated to the final section.
- Residual risk: GitHub's Mermaid renderer may differ from local Mermaid CLI, but the README Lifecycle diagram now uses the simpler state-diagram style already present in project docs.
