# Research

## Research Findings

### Architecture
- Affected modules: documentation-only surface under `README.md`, `docs/installation.md`, and possibly `docs/index.md`.
- Dependency chains: MkDocs reads `mkdocs.yml` nav entries and Markdown files under `docs/`; the README links directly to `docs/installation.md` and the published documentation site.
- Blast radius: low. The likely change is user-facing documentation and governed evidence only; no Go runtime, release pipeline, package manager, or generated adapter behavior needs to change.
- Constraints: keep `docs/installation.md` as the canonical installation matrix; preserve the existing architecture-aware macOS/Linux/Windows release examples; keep generated adapter paths aligned with `docs/ai-tools.md`.

### Patterns
- Existing conventions: docs use direct headings, copyable fenced commands, and stable relative links. `docs/index.md` already identifies Installation as the entry point for "New users, AI coding tools".
- Existing Slipway pattern: `docs/installation.md` already contains a detailed "AI Tool Installation Prompt" that asks the agent to inspect the checkout, detect OS/CPU, prefer project-owned release sources, initialize adapters, avoid overwriting user-owned AI-tool files, and verify with `slipway status --json` plus `git status --short --branch`.
- Microsoft Agent 365 pattern: the docs present AI-guided setup as a first-class workflow where the user opens a project and pastes one prompt into an AI coding agent with terminal access. The CLI reference explicitly distinguishes manual CLI installation from the AI-guided workflow.
- AGENTS.md pattern: agent-specific setup commands and operational context should be predictable and separate enough that agents can find them without cluttering human quick-start prose.
- OpenCode pattern: installation docs pair a platform install matrix with concrete prompt examples and interactive slash-command workflows.
- User-provided oh-my-opencode pattern: the README foregrounds a human-facing copy/paste prompt and a separate "For LLM Agents" instruction. Useful: short prompt plus agent-specific checklist. Unsafe or unsuitable for Slipway: promotional phrasing, branding instructions, and post-install social actions.

### Risks
- Technical risks: low. Documentation-only edits can still break MkDocs links/headings or drift from actual adapter paths.
- Guardrail domains: none. The change does not modify authentication, credentials, payments, schemas, destructive operations, privacy handling, or external API contracts.
- Prompt safety risk: medium if copied patterns encourage agents to perform unrequested marketing, repository starring, browser OAuth, or broad config rewrites. Slipway should explicitly tell agents to stop for missing prerequisites, avoid same-name unverified packages, preserve user-owned AI-tool files, and avoid promotional/social actions.
- Reversibility: high. Documentation edits can be reverted cleanly without state migration.

### Test Strategy
- Existing coverage: docs are covered by `mkdocs build --strict`; Go tests cover CLI contracts but are not directly affected if only Markdown changes.
- Infrastructure needs: no new test helper or fixture required for docs-only work.
- Verification approach: run `mkdocs build --strict`; run `go test ./...` or a focused command only if implementation or generated adapter contracts change; run `go run . validate --json` to check governed readiness.

## Alternatives Considered
- Approach 1: Leave the existing detailed `docs/installation.md#ai-tool-installation-prompt` as-is and only record research. Tradeoff: no user-visible improvement; does not satisfy the request to add the feature if the existing prompt is too buried.
- Approach 2: Add a prominent "Install With An AI Agent" entry near the top of `docs/installation.md` and mirror a short pointer in `README.md`, while keeping the existing long prompt as the canonical detailed checklist. Tradeoff: small duplication, but strongest discoverability and closest to the user-provided oh-my-opencode pattern without adopting unsafe parts.
- Approach 3: Add a new dedicated page such as `docs/agent-install.md` and link it from nav. Tradeoff: cleaner separation, but unnecessary for a small install/onboarding feature and adds another docs surface to keep synchronized.
- Selected: Approach 2. User confirmed this direction on 2026-05-27. This improves first-glance discoverability while preserving the current canonical install matrix and minimizing drift.

## Unknowns
- Resolved: Which comparable projects provide useful AI-agent installation patterns? -> Microsoft Agent 365, AGENTS.md, OpenCode, and the user-provided oh-my-opencode link provide enough positive and negative patterns.
- Resolved: Is Slipway missing the detailed prompt entirely? -> No. `docs/installation.md` already has a detailed AI tool installation prompt; the gap is prominence, short copy/paste entry, and explicit safety framing.
- Remaining: None.

## Assumptions
- Documentation-only scope is sufficient. Evidence: `intent.md` excludes release/package/installer behavior changes, and current docs already contain the long operational prompt.
- `docs/installation.md` should remain canonical. Evidence: `README.md`, `docs/index.md`, and `mkdocs.yml` already make Installation the install/onboarding entry point.
- Safe agent guidance should avoid promotional/social actions. Evidence: the referenced oh-my-opencode install guide contains agent-specific promotional and starring instructions, and a related public issue flags that class of instruction as prompt-injection/social-engineering risk.
- The generated `artifacts/codebase/` snapshot is intentionally retained with
  this archive because the operator wants the codebase context included in the
  eventual commit.

## Canonical References
- `artifacts/changes/archived/investigate-ai-agent-guided-installation-prompts-in-comparable-projects-and-add-equivalent-insta/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `README.md`
- `docs/installation.md`
- `docs/index.md`
- `docs/ai-tools.md`
- `mkdocs.yml`
- https://learn.microsoft.com/en-us/microsoft-agent-365/developer/get-started
- https://learn.microsoft.com/en-us/microsoft-agent-365/developer/agent-365-cli
- https://learn.microsoft.com/en-us/entra/agent-id/agent-id-ai-guided-setup
- https://agents.md/
- https://opencode.ai/docs
- https://github.com/opensoft/oh-my-opencode#installation
- https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/refs/heads/master/docs/guide/installation.md
- https://github.com/code-yeongyu/oh-my-openagent/issues/2071
