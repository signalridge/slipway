# Research

## Research Findings

### Architecture
- Affected modules: `README.md`, `docs/workflow.md`, `internal/stringutil/html.go`, `internal/stringutil/html_test.go`, and `internal/engine/progression/advance_test.go`.
- Dependency chains: `internal/engine/progression/advance_intake.go` delegates Open Questions blocking to `internal/stringutil.HasBlockingOpenQuestions`; `internal/engine/governance/traceability.go` also consumes the same helper for intent-gap detection.
- Blast radius: low and bounded. README/docs changes affect rendered documentation only; Open Questions behavior changes a single shared string utility with focused tests covering intake progression.
- Constraints: preserve `change.yaml` as lifecycle authority, preserve state ordering, and keep unchecked checklist items and substantive unresolved question prose blocking.

### Patterns
- Existing conventions: Markdown docs use fenced Mermaid blocks; focused Go tests live beside the package behavior they cover; Open Questions semantics are documented in `docs/workflow.md`.
- Reusable abstractions: `stringutil.LastMarkdownSectionContent` already isolates the canonical `## Open Questions` section, so the fix belongs in the marker classification inside `HasBlockingOpenQuestions`.
- Convention deviations: none required. The implementation keeps parsing deterministic and avoids adding dependencies.

### Risks
- Technical risks: low. Over-normalizing could hide a real question, so accepted none markers are limited to exact marker phrases after stripping bullet punctuation.
- Guardrail domains: none. This does not touch auth, security credentials, privacy, financial flows, schema migration, irreversible operations, or external API contracts.
- Reversibility: direct revert of the Markdown and helper/test changes restores prior behavior.

### Test Strategy
- Existing coverage: `internal/stringutil/html_test.go` covers helper semantics; `internal/engine/progression/advance_test.go` covers intake routing based on Open Questions content.
- Infrastructure needs: none. Mermaid CLI is already available locally as `mmdc`.
- Verification approach: parse all README/docs Mermaid blocks with `mmdc`; run focused Go tests for Open Questions helper/progression behavior; run broader repo-native tests before closeout.

## Alternatives Considered
- Approach A: Documentation-only workaround using `(none)` everywhere. Tradeoff: avoids code change but leaves the recurring false positive for `- None.` unresolved.
- Approach B: Normalize explicit none markers in `HasBlockingOpenQuestions`, update docs/tests, and keep unchecked checklist/prose blockers unchanged. Tradeoff: small behavior change, but directly fixes the recurring workflow friction with bounded risk.
- Approach C: Change artifact templates only. Tradeoff: helps newly generated artifacts but does not fix manually edited artifacts or existing bundles.
- Selected: Approach B, alongside the requested README Mermaid/status-image changes, because it solves the observed root cause rather than requiring operators to remember one exact spelling.

## Unknowns
- Resolved: Whether existing Mermaid blocks parse locally -> all four Mermaid blocks in `README.md` and `docs/*.md` parsed with `mmdc`.
- Resolved: Why `- None.` caused repeated Open Questions routing -> `HasBlockingOpenQuestions` accepted only `(none)`, empty/comment-only sections, and checked checklist entries.
- Remaining: None.

## Assumptions
- GitHub rendering is stricter or less predictable than local `mmdc` for the README Lifecycle flowchart; replacing it with the same state-diagram style already used in `docs/workflow.md` is the lowest-risk compatibility improvement. Evidence: `README.md`, `docs/workflow.md`, and local `mmdc` parse command.
- The provided `camo.githubusercontent.com` URL should be embedded directly as a README image source. Evidence: user correction and supplied URL.

## Canonical References
- `README.md`
- `docs/workflow.md`
- `internal/stringutil/html.go`
- `internal/stringutil/html_test.go`
- `internal/engine/progression/advance_intake.go`
- `internal/engine/progression/advance_test.go`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
