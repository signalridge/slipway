# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/engine/capability/export.go` renders the workflow-owned skill index. It is informational only and is not read by the Slipway kernel.
  - `internal/engine/capability/surfaces.go` owns public command surfaces and explicit `--focus` aliases.
  - `internal/toolgen/toolgen.go` generates adapter skill trees, workflow command references, and the workflow-owned skill index.
  - `internal/tmpl/templates/skills/workflow/*.tmpl` owns the generated workflow entry skill and command reference prose.
  - `internal/tmpl/templates/skills/*/references/*.md` owns optional hydrate/reference material.
  - `internal/toolgen/toolgen_test.go` and `internal/engine/capability/export_test.go` are the lowest-risk drift guards.
- Dependency chains:
  - `toolgen.Generate` -> `exportedCapabilityRegistry` -> `capability.BuildSkillIndexWithPaths` -> generated `references/skill-index.md`.
  - `toolgen.Generate` -> `renderStandaloneWorkflowCommandReference` -> `buildWorkflowSkillData` -> generated `references/command-reference.md`.
  - `capability.surfacePolicy` -> `ExplicitFocusesForCommand` / `LookupFocus` -> routed command focus discovery and resolution.
- Blast radius: generated skill and reference text plus tests. No state-machine, lifecycle, CLI flag, gate, JSON contract, archive, or runtime command behavior should change.
- Constraints:
  - Preserve the remembered thin-runtime / thick-host / light-audit boundary: runtime stays narrow; host/procedure/reference surfaces carry guidance.
  - Keep `slipway next/run --json` as the governed handoff authority. The index and command reference must remain lookup aids.
  - Do not resurrect retired `references/catalog/` artifacts or export non-allowlisted support skills as host `SKILL.md` files.

### Patterns
- Existing conventions:
  - Capability data is Go-owned and deterministic. Public alias data belongs in `capability.surfacePolicy`.
  - Toolgen templates should reuse Go metadata instead of duplicating command/focus tables manually.
  - Existing tests already enforce frontmatter trigger wording, generated host allowlist size, no retired catalog artifacts, reference byte budgets, hydrate reference resolution, and script parse/executable checks.
- External/current-practice signals:
  - The Agent Skills specification and Claude Code docs emphasize progressive disclosure: concise `SKILL.md` entrypoints, optional `references/`, `scripts/`, and `assets/`, and one-hop file references from the entry skill.
  - The official `anthropics/skills` repository is the canonical reference corpus; it presents skills as self-contained folders with `SKILL.md`, scripts, and resources, plus a warning to test skills before relying on them.
  - Popular GitHub results for `topic:agent-skills pushed:>2026-05-01` show a split between official/reference packs and large marketplace/awesome-list repos. Verified examples include `anthropics/skills`, `ComposioHQ/awesome-claude-skills`, `addyosmani/agent-skills`, `wshobson/agents`, and `github/awesome-copilot`.
  - Community Reddit/X signals are directionally consistent: descriptions drive activation, scripts should stay focused with clear arguments/errors, references should be discoverable without multi-hop chains, and large marketplaces create a quality-validation problem.
- Reusable abstractions:
  - Add a read-only surface-list helper in `internal/engine/capability/surfaces.go` instead of hard-coding focus aliases into templates.
  - Extend `commandEntry` with public focus metadata populated from `ExplicitFocusesForCommand`.
  - Extend the skill index with a compact public-focus section rather than broadening the exported host skill set.
- Convention deviations: none required. All changes can use existing generated-template and unit-test patterns.

### Risks
- Technical risks:
  - Medium: leaking non-exported support skills as host paths would violate the slim generated surface contract. Mitigation: list public focus aliases without host `SKILL.md` paths and keep existing allowlist tests.
  - Medium: duplicating public focus aliases in prose can drift from `surfacePolicy`. Mitigation: render focus data from Go helpers and add generated-output tests.
  - Low: adding too much reference prose can increase context cost. Mitigation: only add navigation to very long reference files and enforce a high threshold.
  - Low: changing template prose could unintentionally imply new CLI semantics. Mitigation: explicitly state lookup/reference-only authority and keep command behavior untouched.
- Guardrail domains: none. This does not alter auth, secrets handling, privacy, financial flows, schema migration, irreversible operations, or external API behavior.
- Reversibility: high. Template and test changes are easy to revert; no persisted runtime state format changes are involved.

### Test Strategy
- Existing coverage:
  - `internal/toolgen` verifies generated skill tree shape, workflow skill/reference generation, frontmatter, no retired catalog artifacts, hydrate references, reference budgets, and scripts.
  - `internal/engine/capability` verifies registry/surface consistency and skill index rendering.
  - `cmd` verifies route/focus CLI surfaces.
- Infrastructure needs: no new test harness. Use existing temporary generated skill roots and capability unit tests.
- Verification approach:
  - Add capability tests that the generated skill index exposes explicit public focus aliases while not exposing retired catalog paths.
  - Add toolgen tests that generated command references include every explicit `--focus` alias from `surfacePolicy`.
  - Add a lightweight reference-usability test: very long reference files must include `## Quick Navigation`, avoiding a broad rewrite of all references.
  - Run focused tests first: `go test ./internal/engine/capability ./internal/toolgen ./cmd`, then full `go test ./...` and `go build ./...` if focused checks pass.

## Alternatives Considered
- A. Import or generate a broad external skill catalog/marketplace layer. Tradeoffs: maximizes breadth, but adds quality/security review burden, prompt bloat, drift risk, and runtime surface ambiguity. Rejected because it over-engineers this project and conflicts with the thin-runtime/thick-host boundary.
- B. Add a new public command or diagnostics mode for skill quality discovery. Tradeoffs: makes the quality surface explicit, but changes product behavior and creates a new contract to support. Rejected because the user asked not to change project functionality.
- C. Selected: tighten generated lookup surfaces and mechanical guardrails only. Render existing public focus aliases into the workflow command reference and skill index, add narrow tests for focus/index/reference usability drift, and add concise navigation to only the longest reference files. This preserves runtime behavior while addressing the real discoverability and quality gaps surfaced by current skills practice.

## Unknowns
- Resolved: Should Slipway absorb popular marketplace/catalog behavior? -> No. Current GitHub/community signals show catalog popularity, but also quality-validation and noise risks. Slipway should absorb small mechanics, not the marketplace shape.
- Resolved: Should non-exported focus-backed skills become host skills? -> No. Existing allowlist and slim surface tests intentionally prevent that; public `--focus` aliases can be listed without host export.
- Resolved: Should we add a new `skill-quality-audit` command? -> No. Tests cover the quality guardrail without adding user-visible behavior.
- Remaining: None.

## Assumptions
- The user has already approved continuing through the Slipway flow to completion, so the least-risk selected alternative can advance without a separate interactive choice. Evidence: user request says to use the Slipway process and complete the optimization.
- Generated skill/index/reference prose is allowed to change when runtime CLI behavior and JSON contracts stay stable. Evidence: `internal/engine/capability/export.go` states the index is informational and not read by the kernel.
- The right optimization level is a small, test-backed surface improvement, not a new catalog/platform layer. Evidence: prior Slipway memory and current research both point to thin-runtime/thick-host plus progressive disclosure.

## Canonical References
- `artifacts/changes/optimize-slipway-skill-surfaces-and-quality-guardrails-without-changing-runtime-behavior/intent.md` for scope, constraints, and acceptance signals.
- `artifacts/codebase/ARCHITECTURE.md` for module boundaries and runtime authority constraints.
- `artifacts/codebase/TESTING.md` for verification strategy.
- `artifacts/codebase/CONCERNS.md` for coupling and drift concerns.
- `internal/engine/capability/export.go` for generated skill-index authority boundary.
- `internal/engine/capability/surfaces.go` for public command/focus policy authority.
- `internal/toolgen/toolgen.go` for generated adapter skill/reference emission.
- `internal/toolgen/toolgen_test.go` for generated skill tree and quality guardrails.
- `internal/engine/capability/export_test.go` for skill-index rendering tests.
- `https://agentskills.io/specification` for Agent Skills structure, progressive disclosure, one-hop file references, and validation concepts.
- `https://docs.claude.com/en/docs/claude-code/skills` for Claude Code skill structure, trigger descriptions, supporting files, and context-cost guidance.
- `https://github.com/anthropics/skills` for the official public skills reference repository.
- `https://github.com/addyosmani/agent-skills` for a popular engineering lifecycle skill-pack pattern using commands, verification gates, anti-rationalization, and references.
- `https://github.com/wshobson/agents` for a popular multi-harness marketplace pattern and its quality-evaluation emphasis.
- `https://github.com/ComposioHQ/awesome-claude-skills` for evidence that broad curated catalogs are popular but broad.
- `https://www.reddit.com/r/ClaudeCode/comments/1rsa200/simplest_guide_to_buildmaster_claude_skills/` for community guidance on focused scripts, trigger metadata, and one-hop references.
- `https://www.reddit.com/r/claude/comments/1rkjqjf/i_built_a_marketplace_for_skillmd_skills_because/` for community friction around finding, formatting, and trusting third-party skills.
- `https://arxiv.org/abs/2605.11418` for the supply-chain risk of registry-facing SKILL.md metadata/instructions.
