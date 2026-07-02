# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/model/config.go`,
  `internal/model/config_catalog.go`, `cmd/next.go`,
  `cmd/next_skill_view.go`, `cmd/next_wave_plan.go`, `cmd/fix.go`,
  generated skill templates under `internal/tmpl/templates/`, command/docs
  references under `docs/`, and tests in `internal/model`, `cmd`,
  `internal/tmpl`, and `internal/toolgen`.
- Dependency chains: `.slipway.yaml` strict decode -> `model.Config` ->
  `ConfigCatalog` / `ConfigSetValue` -> `cmd` next/fix/wave JSON projection ->
  generated host skill instructions.
- Blast radius: public config schema and host-facing JSON contracts. Lifecycle
  state transitions, evidence file formats, and context-origin validation should
  remain unchanged.
- Constraints: configuration must stay typed and discoverable while avoiding a
  provider-specific DSL. Generated capability boundaries remain engine-owned.

### Patterns
- Existing config conventions derive `slipway config list` from `model.Config`
  via yaml tags and use targeted enrichment tables for allowed values and
  descriptions.
- Existing host JSON views are plain structs in `cmd/next.go` and related view
  builders; subagent directives should follow this pattern instead of adding a
  separate renderer.
- Existing governance instructions treat plan-audit, executor fan-out, review
  peers, fixes, and ship verification as the real subagent dispatch seams.
  Plan authoring itself stays in the coordinator session.
- New config should use snake_case yaml/json field names already used by
  `.slipway.yaml` and command JSON.

### Risks
- High: public JSON contract drift. Mitigation: add focused tests for
  `next_skill.subagent`, `review_batch.subagent`, `wave_plan.executor_subagent`,
  and `fix --json contract.subagent`.
- Medium: config drift between strict decoding and `slipway config list/set`.
  Mitigation: catalog tests for new slot leaves and negative tests for removed
  legacy keys.
- Medium: prompt inheritance ambiguity. Mitigation: use
  `session_instructions`, not `prompt`, and document that inheritance applies to
  the delegated session directive only.
- Low: provider-specific needs are under-modeled. Mitigation: `name` points to
  provider-owned native/MCP/skills target or hub; detailed routing stays behind
  the provider.
- Guardrail domains: external API contracts, because adapters/providers consume
  command JSON.
- Reversibility: schema and docs changes are reversible before release; no
  durable state migration is required.

### Test Strategy
- Existing coverage: host capability delegation tests cover required subagent
  availability, but not configurable directive content.
- Add model tests for parse/resolve/validate, including default inheritance,
  slot override, provider enum validation, and rejection of removed keys.
- Add command tests for projection across plan-audit, executor, review, fix, and
  verify dispatch surfaces.
- Add template/toolgen tests for wording drift and docs links.
- Verification approach: focused package tests first, then `go test ./...`,
  `git diff --check`, and any lint/security gates selected by S3.

### Options
- Option A: Keep provider-profile schema. Tradeoff: strong typing for model and
  MCP fields, but it creates a growing provider DSL and makes prompt inheritance
  ambiguous.
- Option B: Configure high-level lifecycle steps. Tradeoff: simpler names, but
  inaccurate because several lifecycle steps never dispatch subagents and
  plan-audit is a real dispatch seam while plan authoring is not.
- Option C: Configure actual dispatch slots with `type`, `name`,
  `session_instructions`, and `timeout`. Tradeoff: less provider-specific
  validation, but a cleaner public schema that matches real host delegation
  seams and leaves hub internals provider-owned.
- Selected: Option C. The user explicitly selected the dispatch-slot model and
  called out that the inherited natural-language field should be session-level
  guidance rather than subagent/profile inheritance.

## Unknowns
- Resolved: whether `plan` should be configurable -> no; plan authoring stays in
  the main session.
- Resolved: whether review substeps should be configurable -> no; expose one
  `review` slot and keep peer fan-out internal.
- Resolved: whether the natural-language field should be called `prompt` -> no;
  use `session_instructions`.
- Remaining: None.

## Assumptions
- Main is the correct implementation baseline for this follow-up change. Evidence:
  the new governed worktree was created from `main`, and current `rg` output
  shows no existing `ConfigSubagents` schema on this branch.
- Provider targets named by `name` are resolved by host/provider adapters, not by
  Slipway core. Evidence: existing host capability surfaces already treat
  subagent execution as host-owned.
- The slot list should remain fixed until a new real dispatch seam exists.
  Evidence: current code has hard-coded delegation seams for plan-audit, wave
  executor, review batch/reviewers, fix, and ship verification.

## Canonical References
- `internal/model/config.go`
- `internal/model/config_catalog.go`
- `cmd/next.go`
- `cmd/next_skill_view.go`
- `cmd/next_wave_plan.go`
- `cmd/fix.go`
- `internal/tmpl/templates/skills/plan-audit/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/ship-verification/SKILL.md.tmpl`
- `artifacts/changes/refactor-subagent-configuration-to-slot-based-session-instru/intent.md`
