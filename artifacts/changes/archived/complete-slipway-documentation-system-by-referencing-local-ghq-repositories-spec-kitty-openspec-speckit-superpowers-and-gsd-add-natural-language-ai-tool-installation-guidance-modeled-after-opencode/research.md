# Research

## Research Findings

### Architecture
- Affected modules: documentation surfaces (`README.md`, `mkdocs.yml`, new tracked `docs/*.md`), adapter generation contracts (`internal/toolgen/toolgen.go`, `internal/toolgen/*_test.go`), lifecycle progression (`internal/engine/progression/advance_intake.go`), governance traceability (`internal/engine/governance/traceability.go`), and Markdown section utilities (`internal/stringutil/html.go`).
- Dependency chains: CLI commands and docs describe adapter/runtime behavior from `internal/toolgen`; governed progression depends on `intent.md` section semantics through `advance_intake.go`; governance readiness depends on traceability checks over the same `intent.md` content.
- Blast radius: low-to-medium. The docs work is user-facing but mostly additive; the runtime fix is narrow and limited to the `## Open Questions` section semantics in intake and traceability. No guardrail domain applies.
- Constraints: keep `change.yaml` as current-state authority and `events/lifecycle.jsonl` as append-only trace; do not introduce a new governance mode; do not move governed change archives out of the originating worktree.
- Architecture review answer: the project needs a targeted contract review, not a rewrite. Evidence: `mkdocs.yml` points at documentation pages that are not tracked in `HEAD`, `README.md` references a docs workflow that is not present in `.github/workflows/`, and `Open Questions` used weaker non-empty-section semantics in intake than traceability had already evolved.

### Patterns
- Existing conventions: Go runtime contracts are protected by focused package tests; adapter surfaces are generated from `internal/toolgen/toolgen.go` and frozen by `internal/toolgen/toolgen_test.go`; governed artifacts live under `artifacts/changes/<slug>/`.
- Reference documentation patterns:
  - spec-kitty uses a Divio-style split across tutorials, how-to guides, reference, and explanation, with docs verification notes for relative links and required pages.
  - OpenSpec and Spec Kit publish explicit AI-tool integration matrices, non-interactive setup commands, generated directory paths, and installation/switch/upgrade caveats.
  - Superpowers uses a natural-language OpenCode installation prompt that tells OpenCode to fetch and follow a canonical install document.
  - GSD presents docs by audience and separates command syntax by runtime spelling.
  - OpenCode documents project command files under `.opencode/commands/`; Slipway uses flat `.opencode/commands/slipway-*.md` files so OpenCode derives stable slash-hyphen command IDs.
- Reusable abstractions: use `stringutil.LastMarkdownSectionContent` as the common Markdown-section primitive and centralize blocking question semantics in a shared helper instead of duplicating intake/traceability logic.
- Convention deviations: adding tracked `docs/` pages will replace the current nav entries rather than preserving placeholder page names, because the current `mkdocs.yml` nav is not backed by tracked files.

### Risks
- Technical risks:
  - Medium: docs may overstate installation methods if they assume unreleased packages; mitigate by separating source install, release artifacts, and `go install github.com/signalridge/slipway@<tag>` where release tags exist.
  - Medium: `Open Questions` semantics can regress if intake and traceability diverge again; mitigate with one shared helper and targeted regression tests.
  - Low: MkDocs configuration may fail if docs pages are missing or nav paths drift; mitigate with tracked pages and strict build verification when MkDocs is available.
  - Low: repo-scoped `codebase-map` writes outside the current worktree in this checkout; keep it advisory for this change and do not include that generated main-checkout artifact in the doc branch.
- Guardrail domains: none. This change touches docs and governance control flow, not auth, secrets, PII, money movement, schema migration, irreversible operations, or external API contracts.
- Reversibility: high. Docs additions and the narrow helper/test change can be reverted independently. The governed artifact bundle remains local to the current worktree.

### Test Strategy
- Existing coverage: `internal/toolgen/toolgen_test.go` covers supported tool IDs, OpenCode flat command paths, Codex global prompts, byte stability, and adapter surface contracts. Progression/governance packages already have tests for lifecycle and traceability behavior.
- New coverage required:
  - `internal/stringutil` unit tests for blocking versus resolved `## Open Questions` content.
  - `internal/engine/progression` regression tests proving `(none)` and checked questions advance, while unchecked checklist/plain bullets block.
  - Existing traceability tests should continue to pass through the shared helper.
  - Documentation validation should include `mkdocs build --strict` when MkDocs is installed, plus Markdown lint/relative-link verification if available through CI.
- Verification approach:
  - Targeted Go regression: `go test ./internal/stringutil ./internal/engine/progression ./internal/engine/governance -run 'TestHasBlockingOpenQuestions|TestAdvanceIntake_OpenQuestionsUseResolvedItemSemantics|TestTraceability.*OpenQuestions' -count=1`.
  - Full repo behavior before closeout: `go test -timeout=20m ./... -count=1` and `go build ./...`.
  - Docs: validate the updated `mkdocs.yml` against tracked `docs/*.md`; run `mkdocs build --strict` if the command is available in the environment.

## Alternatives Considered

- Full project rewrite: Replace the governed lifecycle/docs/adapters with a newly designed documentation and control-plane model. Tradeoff: may address drift broadly, but it is disproportionate to current evidence and risks breaking stable authority boundaries (`change.yaml` and lifecycle trace separation). Rejected.
- README-only documentation patch: Keep `README.md` as the only user-facing doc and delete or ignore the broken MkDocs nav. Tradeoff: fastest path, but it does not satisfy the request for a completed documentation system or AI-tool installation guidance, and it leaves docs CI/MkDocs intent unclear. Rejected.
- Targeted contract review and docs-system completion: Create a tracked `docs/` system, align `mkdocs.yml`, document install/workflow/commands/adapters/operator guidance, add natural-language AI-tool installation guidance, and fix the recurring `Open Questions` blocker with shared semantics and tests. Tradeoff: larger than a README patch, but it directly addresses the observed failures without redesigning the governance architecture. Selected.

## Unknowns
- Resolved: Should the project begin a fundamental review? -> Yes, but scoped to contract drift and documentation surfaces; the architecture does not need a rewrite.
- Resolved: Are docs pages already present? -> No tracked `docs/` tree is present in `HEAD`, while `mkdocs.yml` references pages such as `workflow-test-menu.md` and `agent-contracts.md`.
- Resolved: Is a docs workflow present? -> No `.github/workflows/docs.yml` is tracked, despite `README.md` describing one.
- Resolved: Does `Open Questions` need runtime remediation? -> Yes. `(none)` and checked items should not block; unchecked or plain bullet questions should block.
- Remaining: Whether to add a dedicated docs workflow in this change or only fix docs/config and leave CI docs publishing for a follow-up. Current recommendation: add or correct docs verification if it can be done without broad CI redesign.


## Assumptions
- Slipway should keep docs in English for the public repository unless the user asks for localized docs. Evidence: current `README.md`, `mkdocs.yml`, command help, and reference repos used for structure are English-first.
- Natural-language AI-tool installation guidance should be precise enough for OpenCode/Codex/Claude-style agents but should not ask an agent to perform destructive or ambiguous install steps. Evidence: Superpowers' OpenCode install prompt pattern and OpenCode's documented project command directory model.
- Source and release installation can both be documented, but release-channel claims must match repository release configuration. Evidence: `go.mod` module path is `github.com/signalridge/slipway`, and `.goreleaser.yaml` publishes release artifacts with a footer showing Homebrew Cask and `go install` commands.
- The Open Questions fix belongs in the current change because the governed docs work cannot progress reliably while the intake section is semantically resolved but textually non-empty. Evidence: S0 intake advanced from `research` to `confirm` after centralizing blocking semantics.


## Canonical References
- `artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/intent.md` for the original request and intake context.
- `README.md` for existing command surface, workflow, verification, release, authority, and skill-surface descriptions.
- `mkdocs.yml` for current documentation site configuration and broken nav target inventory.
- `.github/workflows/ci.yml` and `.goreleaser.yaml` for current verification and release/install claims.
- `internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, and `internal/toolgen/adapter_contract_test.go` for supported AI tool adapters and generated path contracts.
- `internal/engine/progression/advance_intake.go`, `internal/engine/governance/traceability.go`, and `internal/stringutil/html.go` for `Open Questions` behavior.
- `spec-kitty/docs/index.md` for docs categories and docs verification notes.
- `spec-kitty/docs/reference/supported-agents.md` and `spec-kitty/docs/how-to/harnesses/opencode.md` for AI harness directory and OpenCode setup patterns.
- `OpenSpec/docs/supported-tools.md` and `OpenSpec/docs/workflows.md` for tool matrices and workflow documentation patterns.
- `spec-kit/docs/installation.md` and `spec-kit/docs/reference/integrations.md` for install and integration safety patterns.
- `superpowers/README.md` and `superpowers/docs/README.opencode.md` for natural-language OpenCode installation guidance.
- `get-shit-done/docs/README.md`, `get-shit-done/docs/COMMANDS.md`, and `get-shit-done/docs/USER-GUIDE.md` for audience-indexed docs and runtime-specific command syntax.
- `opencode/README.md` for OpenCode install and project command path behavior.
- `artifacts/codebase/ARCHITECTURE.md`, `artifacts/codebase/TESTING.md`, and `artifacts/codebase/CONCERNS.md` generated by `go run . codebase-map --json` as advisory repo-scoped context.
