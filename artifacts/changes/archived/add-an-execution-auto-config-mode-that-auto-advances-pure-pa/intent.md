# Intent

## Summary
Add an execution.auto config mode that auto-advances pure-pacing lifecycle confirmations (auto-confirm preset to the suggested value, downgrade review-batch and non-sensitive skill-handoff host-confirmation to standing authorization, auto-acknowledge human-verify checkpoints) while keeping all sensitive/guardrail-domain confirmations, the intake Approved Summary final review, decision-type checkpoints, and every evidence gate fully fail-closed.
## Complexity Assessment
complex
<!-- Rationale: touches the lifecycle authority (preset gate, confirmation policy,
     run loop) plus config schema, generated skill surfaces, and CLI flags; the
     sensitive-domain fail-closed boundary is safety-critical and must be proven
     by tests across multiple packages. -->

## Guardrail Domains
none detected — the work does not touch auth/authz, credentials/PII, financial,
schema migration, irreversible ops, or external API data contracts. It does
change CLI/JSON/generated-skill external contracts, which are reviewed as
contracts at S3.

## In Scope
- `internal/model/config.go`: add `Auto bool` to `ConfigExecution`
  (`auto,omitempty`), with Normalize default (false), Validate (no new
  constraint), conditional ToYAML emit, and an `AutoEnabled()` accessor.
- `cmd/run.go`: add `--auto` / `--no-auto` override flags; resolve effective
  auto = flag-override-else-config (flag > config); thread the resolved value
  into the governed loop and view building.
- Engine preset auto-confirm: in `internal/engine/progression/advance_governed.go`
  preset gate (`PresetConfirmationBlockers` path), when auto is effective,
  auto-confirm the pending preset to the suggested/effective preset
  (upgrade-only — never a downgrade) and record a lifecycle event instead of
  surfacing a blocked view.
- Host confirmation policy: in `cmd/next.go` `deriveConfirmationRequirement`,
  when auto is effective, downgrade `review_batch` and non-sensitive
  `skill_handoff` boundaries from hard_stop to prior-authorization-sufficient.
- Human-verify checkpoint auto-ack: in `cmd/run.go` `runGovernedLoop` (or the
  checkpoint resume path), when auto is effective and the active checkpoint kind
  is human-verify, auto-inject an acknowledgment so the loop continues.
- Generated slipway skill surfaces (toolgen templates + emitted SKILL.md): teach
  the host to proceed on standing authorization through pure-pacing boundaries
  in auto mode, while still stopping at the retained human stops.
- Tests across config, engine progression, and cmd, plus README /
  command-reference contract updates where behavior is documented.

## Out of Scope
- Sensitive / guardrail-domain confirmations: never auto — they stay fail-closed
  even with auto on (explicit exclusion).
- Intake `## Approved Summary` final review: never auto-written under auto mode.
- Decision-type checkpoints: never auto-resolved.
- Evidence gates: never weakened — missing or stale evidence still blocks; no
  evidence fabrication, no new force-close/bypass/private-attestation path.
- The `light` preset auto-pass semantics: unchanged (orthogonal feature).

## Constraints
- Must honor project CLAUDE.md: sensitive-domain work fails closed; CLI, JSON,
  generated skills, and docs are external contracts reviewed when behavior
  changes.
- Prefer the smallest clean design that makes the end state true.
- Auto defaults to false (opt-in); a per-run flag overrides config.

## Acceptance Signals
- `go build ./...` and `go test ./...` are green.
- Config round-trips `auto` through `ParseConfigYAML` ↔ `ToYAML`.
- Auto ON: preset confirmation auto-resolves to the suggested preset
  (upgrade-only) and advancement proceeds without a preset hard_stop.
- Auto ON: `review_batch` and non-sensitive `skill_handoff` confirmation
  requirements report `prior_authorization_sufficient` (not a fresh hard_stop).
- Auto ON: a human-verify checkpoint auto-acknowledges; a decision-type
  checkpoint still requires an explicit response.
- RED LINE (test-pinned): with auto ON and a guardrail/sensitive domain, the
  confirmation stays hard_stop and every evidence gate still blocks on missing
  evidence.
- `--no-auto` overrides `execution.auto: true`; `--auto` overrides config false.

## Open Questions
None.

## Deferred Ideas
- Granular per-boundary auto toggles (e.g. `auto_preset` / `auto_review_dispatch`
  / `auto_checkpoint`) — deferred; single switch chosen for the smallest clean
  surface.

## Approved Summary
Add `execution.auto` (default off, opt-in) plus `slipway run --auto/--no-auto`
per-run override (flag > config). When auto is effective, the lifecycle
auto-advances three pure-pacing pauses: preset is auto-confirmed to the
suggested value (upgrade-only, never a downgrade); `review_batch` and
non-sensitive `skill_handoff` host-confirmation drops to standing authorization;
and human-verify checkpoints auto-acknowledge.

Scope boundary (held fail-closed even with auto on): sensitive/guardrail-domain
confirmations, the intake Approved Summary final review, decision-type
checkpoints, and every evidence gate are never auto-advanced; no force-close,
bypass, or private-attestation path is introduced; the `light` preset auto-pass
semantics are unchanged.

Primary acceptance signal: `go test ./...` is green and tests pin the red line —
with auto on and a guardrail domain the confirmation stays hard_stop and missing
evidence still blocks, and `--no-auto` overrides `execution.auto: true`.

Confirmed by user on 2026-06-20.
