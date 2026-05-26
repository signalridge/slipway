# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI, Markdown documentation
- Languages: Go, Markdown
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Complete Slipway documentation system by referencing local ghq repositories spec-kitty, openspec, speckit, superpowers, and gsd; add natural-language AI tool installation guidance modeled after opencode.
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Audit Slipway's current tracked documentation surface (`README.md`,
  `mkdocs.yml`, and any created `docs/` pages) against the requested doc-system
  outcome.
- Study local ghq reference repositories for documentation and AI-tool
  onboarding patterns:
  - `/Users/yixianlu/ghq/github.com/Priivacy-ai/spec-kitty`
  - `/Users/yixianlu/ghq/github.com/Fission-AI/OpenSpec`
  - `/Users/yixianlu/ghq/github.com/github/spec-kit`
  - `/Users/yixianlu/ghq/github.com/obra/superpowers`
  - `/Users/yixianlu/ghq/github.com/gsd-build/get-shit-done`
  - `/Users/yixianlu/ghq/github.com/opencode-ai/opencode`
- Create or update the tracked Slipway documentation system so MkDocs has
  real pages for concept, installation, workflow, command, adapter, and
  contributor/operator documentation.
- Add natural-language installation guidance that an AI coding tool can follow
  to install and initialize Slipway, using opencode as a reference for the
  expected AI-facing onboarding shape.
- Fix the recurring `Open Questions` governance blocker so resolved entries
  such as `(none)` and `- [x] ...` no longer count as unresolved intake
  questions.

## Out of Scope
- Do not change Slipway runtime behavior, command semantics, generated adapter
  output, or release packaging unless documentation verification exposes a
  directly blocking inconsistency.
- Do not broaden the `Open Questions` fix into a lifecycle rewrite, artifact
  schema redesign, or new governance mode.
- Do not vendor or copy reference-repository docs verbatim; use them as design
  inputs and cite local observations in research artifacts.
- Do not add network-dependent installation automation in this change.

## Constraints
- Work must stay inside the dedicated governed worktree for this change.
- Documentation should match the current tracked repository state, not implied
  future features.
- Existing repo-native verification remains authoritative: `go test ./...`,
  `go build ./...`, and documentation build checks when available.

## Acceptance Signals
- `mkdocs.yml` points only to tracked Markdown pages that exist in the current
  repository.
- The docs include a coherent user path from "what Slipway is" through
  installation/init, governed workflow, command reference, adapter/tool usage,
  and contributing/operator guidance.
- The install docs include an AI-tool-readable natural-language prompt or
  procedure for installing and initializing Slipway, explicitly referencing
  supported tool adapters including opencode.
- Research artifacts record the local reference repositories inspected and the
  concrete patterns borrowed from each.
- Repo-native checks and applicable docs build checks pass from this worktree.
- Targeted regression tests prove that only unresolved Open Questions entries
  block intake progression, while `(none)` and checked entries do not.

## Research Resolution Notes
- The final documentation tree should cover both public user docs and
  AI/operator docs. Public docs should be the tracked MkDocs surface; AI
  installation guidance should be a first-class install page.
- Reference repository patterns should be borrowed as structure and discipline,
  not copied verbatim: Divio-style categories, tool/adapter matrices,
  workflow/command references, verification-before-completion rules, and
  claim-checking for generated docs.
- Applicable docs verification should include MkDocs strict build when the
  Python docs toolchain is available, plus repo-native Go build/test checks.

## Open Questions
(none)

## Deferred Ideas
- Automating installation into external package managers or modifying release
  distribution channels.
- Changing generated AI adapter templates beyond documenting their current
  initialization flow.

## Approved Summary
Confirmed 2026-05-26T14:52:55Z from the continued goal context preserving
the full original objective.

Complete Slipway's tracked documentation system by studying local ghq reference
repositories (`spec-kitty`, `OpenSpec`, `spec-kit`, `superpowers`, `gsd`, and
`opencode`) and applying the patterns that fit Slipway's current runtime
contract. The change will create a coherent MkDocs documentation tree covering
concepts, installation/init, governed workflow, commands, AI tool adapters,
operator/contributor guidance, and troubleshooting. It will also add a
natural-language, AI-tool-readable installation guide for installing and
initializing Slipway with supported adapters including opencode. Runtime
behavior, command semantics, adapter generation, release packaging, and
network-dependent install automation are out of scope unless documentation
verification exposes a directly blocking inconsistency. Scope was amended to
include the narrow `Open Questions` blocker fix requested on 2026-05-26: checked
or explicitly empty open-question entries must not keep intake/research blocked.
Primary acceptance is that MkDocs references only tracked pages, the docs form
a complete user and operator path, research records the local reference
evidence, the blocker fix has targeted regression coverage, and repo-native
build/test/docs checks pass from this worktree.
