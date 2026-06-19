# Intent

## Summary
expand Slipway AI tool adapters for common modern coding tools

## Complexity Assessment
complex
Rationale: this changes generated adapter contracts, refresh ownership, docs,
and tests across multiple host layouts. Some target hosts have materially
different command, skill, agent, hook, and settings surfaces.

## Guardrail Domains
None detected.

## In Scope
- Define an implementation standard for new AI tool adapters based on current
  Slipway contracts plus investigated patterns from `gsd-core` and Trellis.
- Investigate common target tools, including `kiro`, `copilot`, `qwen`, and
  similar current coding agents, then select a practical first implementation
  set from the evidence.
- Treat `pi` as the only must-have adapter target for this change.
- Extend Slipway's generated adapter registry, deterministic file generation,
  refresh ownership, auto-detection, tests, docs, and surface manifest for the
  implemented tools.
- Preserve Slipway's thin-adapter rule: generated host files route agents back
  to the CLI and do not become separate lifecycle engines.

## Out of Scope
- Replacing Slipway's compiled fail-closed lifecycle with Markdown-only
  workflows, external daemons, or host-specific governance logic.
- Adding agent orchestration, marketplace, memory, or channel runtime features
  copied from Trellis/GSD unless strictly necessary for adapter discovery.
- Treating lower-priority tools as blocking when they require a distinct
  product capability beyond the first implementation standard.

## Constraints
- Use current worktree CLI behavior as lifecycle authority.
- Keep generated adapters deterministic and owned by Slipway markers/manifests
  so refresh can preserve user-owned files.
- Prefer a declarative adapter capability model over scattered host-specific
  conditionals.
- Do not add tool surfaces that cannot be verified locally by generation tests.

## Acceptance Signals
- `slipway init --tools pi` generates the expected adapter surfaces in a
  temporary workspace.
- Any additional common tools selected from the investigation generate their
  expected adapter surfaces in a temporary workspace.
- `slipway init --tools all` includes the implemented tools and remains sorted,
  deterministic, and documented.
- Refresh and auto-detection tests cover the new tool roots without treating
  bare host directories as Slipway-owned.
- Toolgen contract tests and `docs/SURFACE-MANIFEST.json` are updated and pass.
- Relevant docs name the new tool IDs, generated paths, invocation style, and
  refresh behavior.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Must-have set corrected: `pi` is the only required target; `kiro`,
  `copilot`, `qwen`, and similar tools are investigation candidates.

## Deferred Ideas
- Later broaden to additional common hosts after the adapter capability model
  proves stable.
- Consider Trellis/GSD-style memory, channel, marketplace, and agent-pack
  features separately from thin adapter support.

## Approved Summary
Confirmed by user on 2026-06-19T05:18:05Z.

This change will investigate common modern AI coding tools using `gsd-core`,
Trellis, and current Slipway adapter contracts, define a standard for adding
thin Slipway adapters, and implement the practical first set. `pi` is the only
required adapter target for this change. `kiro`, `copilot`, `qwen`, and similar
tools are investigation candidates, not pre-declared must-haves. Implemented
adapters must route agents back to the Slipway CLI, keep generated files
deterministic and refresh-owned, update docs and the surface manifest, and pass
toolgen verification. Marketplace, memory, channel runtime, and independent
host-specific governance engines are out of scope.
