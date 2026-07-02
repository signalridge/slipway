# Decision

## Alternatives Considered

1. Keep a provider-profile schema with typed provider fields. This would make
   `model`, `server`, `tool`, `skill`, and `entrypoint` explicit in Slipway,
   but it would also turn Slipway config into a provider DSL and would still be
   incomplete for hub-style MCP/skills providers.
2. Configure high-level lifecycle steps such as `plan`, `implement`, `review`,
   `fix`, and `verify`. This is simple, but inaccurate: plan authoring should
   stay in the main session, while `plan_audit` is the actual subagent dispatch
   seam.
3. Configure actual delegation slots with a small envelope:
   `type`, `name`, `session_instructions`, and `timeout`.

Selected: option 3.

## Selected Approach

Add `Config.Subagents` with fixed slots:

```yaml
subagents:
  default:
  plan_audit:
  executor:
  review:
  fix:
  verify:
```

Each slot has:

```yaml
type: native | mcp | skills
name: <provider target>
session_instructions: <optional delegated-session guidance>
timeout: <optional duration string>
```

`default` is optional and is merged into specific slots. `type` defaults to
`native`. `session_instructions` is intentionally named as session guidance: it
is not a provider profile prompt, not a subagent definition, and not a tool
permission policy.

Review substeps are not configurable. `review` is the single configured review
dispatch slot; Slipway and the selected provider/hub own internal review peer
fan-out. `plan` is not configurable because plan authoring remains in the main
session; only `plan_audit` is a subagent delegation slot.

## Interfaces and Data Flow

- `.slipway.yaml` strict decode gains `subagents`.
- `ConfigCatalog` exposes scalar leaves:
  `subagents.<slot>.type`, `subagents.<slot>.name`,
  `subagents.<slot>.session_instructions`, and `subagents.<slot>.timeout`.
- `Config.ResolveSubagent(slot)` returns a resolved directive with generated
  capabilities.
- `cmd` view builders attach directives to:
  `next_skill.subagent` for plan-audit and ship-verification,
  `input_context.wave_plan.executor_subagent`,
  `review_batch.subagent`, and `fix --json contract.subagent`.
- Generated skill templates, including plan-audit, executor, review peers, and
  ship verification, explain that hosts should pass the resolved directive to
  provider/native/MCP/skills dispatch where present.

## Rollout and Rollback

Rollout is additive until release: add the schema, projection, docs, and tests
in one PR. No durable state migration is required because the schema is a
project config surface.

Rollback is to remove `Config.Subagents`, the catalog entries, command
projection fields, docs/templates, and tests in the same PR before release.
Verification command: `go test ./internal/model ./cmd ./internal/tmpl
./internal/toolgen -count=1` plus `git diff --check`.

## Risk

- Public JSON consumers may depend on field names. Mitigation: add only new
  `subagent` fields and keep existing host capability blockers unchanged.
- Config users may assume provider-specific args belong in Slipway. Mitigation:
  docs explain that hub/provider internals live behind `name` and
  `session_instructions`.
- `session_instructions` inheritance may still be overused. Mitigation: docs
  advise keeping `default.session_instructions` short and putting slot-specific
  guidance on concrete slots.
- Review batch placement must avoid per-reviewer config resurfacing. Mitigation:
  the command contract uses one `review_batch.subagent` directive while
  generated review-peer templates explain that the shared review slot may point
  at a native agent, MCP target, or skills hub.
