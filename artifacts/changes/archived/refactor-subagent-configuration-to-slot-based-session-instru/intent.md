# Intent

## Summary
Refactor subagent configuration to slot-based session instruction model
## Complexity Assessment
complex
The change touches the public configuration schema, host-facing JSON
projection, generated skill instructions, documentation, and tests.

## Guardrail Domains
External API contracts: host-facing subagent directive JSON is a contract with
adapter/provider surfaces.

## In Scope
- Replace the old `subagent_provider_profiles` plus role `profile/prompt`
  design with slot-based subagent delegation config.
- Support only these configurable slots under `subagents`: `default`,
  `plan_audit`, `executor`, `review`, `fix`, and `verify`.
- Configure each slot with `type`, `name`, optional `session_instructions`,
  and optional `timeout`.
- Treat `session_instructions` as delegated-session guidance that can be
  inherited from `default`, not as provider/subagent profile inheritance.
- Update config parsing, validation, catalog/list/set surfaces, host JSON
  projection, docs, Chinese docs, generated skill templates, and tests.

## Out of Scope
- Do not add a configurable `plan` slot; plan authoring remains in the main
  session.
- Do not expose review substeps such as `security_review` or
  `code_quality_review` as user-configurable subagent slots.
- Do not expose user-configurable `tool_policy`, `allowed_skills`, or
  `allowed_mcp_servers`.
- Do not model provider-specific arguments as Slipway typed fields or arbitrary
  profile maps.

## Constraints
- Preserve Slipway-owned generated capability boundaries in host directives.
- Keep native, MCP, and skills mutually exclusive provider families.
- Keep the public configuration surface documentable and fail closed on removed
  legacy keys.
- Preserve unrelated worktree changes outside this follow-up change.

## Acceptance Signals
- `config` parse/validate/catalog supports the new slot schema and rejects the
  removed profile schema.
- `next`, `fix`, wave executor, review batch, plan-audit, and verify host
  projections expose the new `type/name/session_instructions/timeout`
  directive shape.
- English and Chinese subagent docs plus generated skill templates describe the
  same schema.
- Focused model/cmd/template tests pass, followed by broader Go verification
  appropriate for the touched surface.

## Open Questions
None

## Deferred Ideas
- A future `plan_research` slot can be considered if Slipway adds a real
  advisory research dispatch point distinct from plan authoring.
- Provider-owned hub schemas may evolve independently behind `name` and
  `session_instructions`.

## Approved Summary
User confirmed on 2026-07-01: refactor subagent configuration from the old
`subagent_provider_profiles` and `profile/prompt` design to a slot-based
delegation model with `default`, `plan_audit`, `executor`, `review`, `fix`,
and `verify`. Each slot uses `type`, `name`, optional
`session_instructions`, and optional `timeout`. The change intentionally does
not expose a `plan` slot, review substep slots, arbitrary provider profile
fields, or user-configurable tool allowlists/policies.
