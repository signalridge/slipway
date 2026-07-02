# Intent

## Summary
Fix runtime-host environment wiring discoverability for issue #395, and
systematically investigate whether similar hidden environment-contract gaps
exist across Slipway public surfaces.

## Complexity Assessment
complex
Rationale: this changes a public host-integration contract surface, may extend
the env catalog schema/output, requires repo-wide contract-gap research, and
must preserve existing runtime semantics while improving discoverability.

## Guardrail Domains
external_api_contracts

## In Scope
- Investigate and repair the host-facing wiring surface for
  `SLIPWAY_HOST_CAPABILITIES` and `SLIPWAY_HOST_CAPABILITY_FALLBACKS`.
- Extend the env catalog or equivalent single-authority surface so
  `slipway config list --env` JSON/text output can expose accepted values,
  value-to-behavior mappings, unset behavior, and host integration guidance
  where an environment variable has a real operational contract.
- Audit all Slipway environment variables across runtime-host, secret, and
  repo-policy scopes for the same class of hidden contract gap.
- Add a host-integration doc that reconciles generated skill preconditions such
  as "host declared subagent capability" with the concrete env knob.
- Add contract tests for env catalog completeness, output shape, and the
  documented host capability wiring.

## Out of Scope
- Reworking subagent execution, dispatch, or capability-resolution semantics.
- Changing accepted capability tokens unless research proves the existing
  runtime contract conflicts with the public contract.
- Fixing unrelated open issues #371, #361, or #392.
- Publishing, uploading, or editing README video assets.

## Constraints
- Preserve existing behavior for all environment variables unless the current
  behavior is inconsistent with the newly documented public contract.
- Keep skills focused on agent workflow requirements; put host-integration
  manuals in catalog/docs surfaces instead of per-stage skill prose.
- Treat the env catalog as the preferred single public authority for environment
  variables consumed by Slipway commands.

## Acceptance Signals
- Without reading Go source, a host integrator can learn from public surfaces how
  to declare subagent capability, including `SLIPWAY_HOST_CAPABILITIES=subagent`
  and the behavior of `delegation`, `none`, `unavailable`, empty, and unset.
- `slipway config list --env` text output and `--json` output expose the
  relevant env wiring contract for runtime-host variables that have accepted
  values or non-obvious unset behavior.
- The same hidden-contract class is investigated across all env vars, with
  fixed gaps or an explicit no-fix-needed rationale.
- Tests cover env catalog contract completeness, config env output, and the
  host capability wiring public contract.
- Governed plan, implementation evidence, review, and final verification pass.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which current env vars have accepted values, format constraints, fallback
  semantics, or behavior mappings that are only discoverable from Go source or
  tests?
- [x] Which public surfaces should own the host-integration contract without
  turning generated workflow skills into host manuals?
- [x] What schema/output design gives host integrators enough detail while
  preserving backward-compatible JSON consumers?

## Deferred Ideas
- A broader redesign of auto mode or plan-audit semantics belongs to #361/#371,
  not this change.
- Future CLI affordances may validate env var values directly, but this change
  focuses on discoverability and contract documentation.

## Approved Summary
Confirmed by user on 2026-07-02: solve issue #395 and, before designing the
fix, thoroughly investigate similar hidden environment-contract problems across
the project. The change will repair host-facing runtime env wiring
discoverability, especially for host capability declarations, by updating the
env catalog/config output/docs/tests while preserving runtime behavior and
leaving unrelated issues #371, #361, and #392 out of scope.
