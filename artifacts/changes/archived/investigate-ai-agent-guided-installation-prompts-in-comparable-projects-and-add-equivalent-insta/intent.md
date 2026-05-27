# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go, MkDocs
- Languages: Go, Markdown
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
investigate AI-agent-guided installation prompts in comparable projects and add equivalent install guidance to Slipway
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Investigate comparable projects that publish AI-agent-oriented installation or onboarding prompts.
- Add equivalent Slipway documentation so an AI agent can install or initialize Slipway from a repository checkout using concrete, copyable instructions.
- Keep the guidance aligned with current Slipway command surfaces and generated adapter paths.

## Out of Scope
- Do not change Slipway binary distribution, package publishing, or release automation.
- Do not add a new interactive installer or change existing CLI install/init behavior.
- Do not broaden this into a full documentation rewrite beyond the installation/onboarding surface needed for agent guidance.

## Constraints
- Preserve repository documentation conventions and existing architecture-aware install examples.
- Verify documentation renders through the repo-native documentation build.
- Use governed Slipway evidence for intake, research, planning, review, and final closeout.

## Acceptance Signals
- Research notes identify relevant external patterns and justify the selected Slipway documentation shape.
- Slipway installation/onboarding docs include a copyable AI-agent prompt or equivalent instructions.
- `mkdocs build --strict` passes after documentation changes.
- Relevant Go tests or validation commands pass if code or generated surfaces change.

## Open Questions
- [x] Which comparable projects provide the clearest AI-agent install/onboarding prompt patterns? Resolved: use Microsoft Agent 365 AI-guided setup as the primary positive pattern, AGENTS.md as the broader agent-context pattern, OpenCode as the install-matrix/prompt-detail pattern, and a documented oh-my-openagent prompt-injection issue as a safety anti-pattern.
- [x] Should Slipway place the guidance directly in `docs/installation.md` only, or also link it from another docs entry point? Resolved: keep the canonical prompt in `docs/installation.md`; evaluate a small README or docs index pointer during planning if discoverability is weak.

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
- Confirmed 2026-05-27T13:17:31Z: Add installation/onboarding documentation that gives AI agents concrete, copyable instructions for installing or initializing Slipway, informed by comparable project patterns. Keep the scope to docs and supporting generated surfaces only if needed; exclude package distribution changes, release automation, and new installer behavior. Primary acceptance is researched rationale plus passing documentation build.
