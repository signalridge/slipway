# Requirements

## Requirements

### Requirement: Opt-in execution.auto config field
REQ-001: The system MUST expose an `execution.auto` boolean configuration field
that defaults to `false` (opt-in) and round-trips losslessly through
`ParseConfigYAML` and `ToYAML`. The field MUST be emitted only when enabled
(`omitempty`), MUST add no new validation constraint, and MUST be exposed through
an `AutoEnabled()` accessor on `ConfigExecution`.

#### Scenario: Default is off and absent on round-trip
GIVEN a project config that does not set `execution.auto`
WHEN the config is parsed and re-encoded with `ToYAML`
THEN `ConfigExecution.AutoEnabled()` is `false`
AND the re-encoded YAML omits the `auto` key.

#### Scenario: Enabled value round-trips
GIVEN a project config containing `execution:\n  auto: true`
WHEN the config is parsed and re-encoded with `ToYAML`
THEN `ConfigExecution.AutoEnabled()` is `true`
AND the re-encoded YAML contains `auto: true` under `execution`.

### Requirement: Per-run auto override flags
REQ-002: The `slipway run` command MUST accept `--auto` and `--no-auto` flags that
override `execution.auto` for that run only (flag takes precedence over config),
and the effective value MUST be config-only when neither flag is passed. `--auto`
and `--no-auto` MUST be mutually exclusive.

#### Scenario: --no-auto overrides config true
GIVEN `execution.auto: true` in config
WHEN `slipway run --no-auto` is invoked
THEN the effective auto value for that run is `false`.

#### Scenario: --auto overrides config false
GIVEN `execution.auto` unset or false in config
WHEN `slipway run --auto` is invoked
THEN the effective auto value for that run is `true`.

#### Scenario: No flag falls back to config
GIVEN `execution.auto: true` in config
WHEN `slipway run` is invoked with neither `--auto` nor `--no-auto`
THEN the effective auto value for that run is `true`.

### Requirement: Auto preset confirmation is upgrade-only
REQ-003: When auto is effective and a workflow-preset confirmation is pending, the
engine MUST auto-confirm the pending preset to the suggested/effective preset
(upgrade-only — the confirmed preset rank MUST be greater than or equal to the
current rank; the engine MUST NEVER auto-downgrade a preset), MUST record a
lifecycle event distinguishing the auto confirmation, and MUST then continue
advancement instead of surfacing a preset hard-stop.

#### Scenario: Auto-confirm to the suggested preset and proceed
GIVEN a change with a pending preset confirmation whose suggested/effective
preset is at least the current preset
AND auto is effective for the advancement
WHEN the engine advances the change
THEN the pending preset is confirmed to the suggested/effective preset
AND a lifecycle event records the auto preset confirmation
AND advancement proceeds without returning a preset confirmation blocker.

#### Scenario: Never downgrades
GIVEN a change whose current confirmed preset already outranks the suggested
preset
AND auto is effective
WHEN the engine evaluates the preset gate
THEN the confirmed preset is not lowered below the current preset.

### Requirement: Auto downgrades pure-pacing host confirmations
REQ-004: When auto is effective and the change is NOT in a guardrail/sensitive
domain, `deriveConfirmationRequirement` MUST downgrade the `review_batch`
confirmation and the non-sensitive `skill_handoff` confirmation from a `hard_stop`
to a standing-authorization boundary where `prior_authorization_sufficient` is
true and `fresh_confirmation_required` is false, while preserving the next action
(run the review batch / run the named skill and record evidence).

#### Scenario: Review batch downgraded under auto
GIVEN auto is effective and the change has no guardrail domain
AND the view presents a non-empty review batch
WHEN the confirmation requirement is derived
THEN the boundary is not `hard_stop`
AND `prior_authorization_sufficient` is true
AND `fresh_confirmation_required` is false.

#### Scenario: Non-sensitive skill handoff downgraded under auto
GIVEN auto is effective and the change has no guardrail domain
AND the view presents a next-skill handoff
WHEN the confirmation requirement is derived
THEN the boundary is not `hard_stop`
AND `prior_authorization_sufficient` is true.

### Requirement: Auto-acknowledge human-verify checkpoints only
REQ-005: When auto is effective and the change is NOT in a guardrail/sensitive
domain, the `slipway run` loop MUST auto-acknowledge an active `human_verify`
checkpoint by injecting a resume acknowledgment so the loop continues. The loop
MUST NOT auto-resolve a `decision` checkpoint or a `human_action` checkpoint;
those MUST continue to require an explicit operator `--resume-response`.

#### Scenario: human-verify auto-acknowledged
GIVEN auto is effective, no guardrail domain, and an active `human_verify`
checkpoint with no operator response
WHEN `slipway run` executes the governed loop
THEN a resume acknowledgment is injected for the checkpoint
AND the loop continues past the checkpoint.

#### Scenario: decision checkpoint stays manual
GIVEN auto is effective and an active `decision` checkpoint
WHEN `slipway run` executes without a `--resume-response`
THEN the checkpoint is not auto-resolved
AND the loop stops awaiting an explicit response.

### Requirement: Sensitive-domain confirmations stay fail-closed under auto
REQ-006: With auto effective and the change in a guardrail/sensitive domain, every
host-confirmation boundary (`review_batch`, `skill_handoff`, and an active
`human_verify` checkpoint) MUST remain a `hard_stop`, and every evidence gate MUST
continue to block on missing or stale evidence. Auto mode MUST NOT introduce any
force-close, bypass, or private-attestation path, and MUST NOT weaken any
evidence gate.

#### Scenario: Guardrail skill handoff stays hard_stop
GIVEN auto is effective and the change has a non-empty guardrail domain
AND the view presents a next-skill handoff
WHEN the confirmation requirement is derived
THEN the boundary is `hard_stop`
AND `prior_authorization_sufficient` is false.

#### Scenario: Guardrail human-verify checkpoint is not auto-acknowledged
GIVEN auto is effective and the change has a non-empty guardrail domain
AND an active `human_verify` checkpoint with no operator response
WHEN `slipway run` executes the governed loop
THEN the checkpoint is not auto-acknowledged
AND the loop stops awaiting an explicit response.

#### Scenario: Missing evidence still blocks under auto
GIVEN auto is effective and a required governance skill has no recorded evidence
WHEN advancement is attempted
THEN the change is still blocked on the missing evidence.

### Requirement: Auto preserves intake summary, evidence, and light auto-pass
REQ-007: Auto mode MUST NOT auto-write the intake `## Approved Summary`, MUST NOT
auto-resolve decision-type checkpoints, MUST NOT fabricate or restamp any
evidence, and MUST leave the existing `light` preset auto-pass semantics
unchanged.

#### Scenario: Approved Summary is never authored by auto
GIVEN auto is effective during intake
WHEN the governed loop reaches the intake Approved Summary boundary
THEN the Approved Summary is not auto-written
AND the boundary still requires operator authorship.

#### Scenario: Light auto-pass unchanged
GIVEN auto is effective on a `light`-preset change
WHEN auto-pass eligibility is evaluated
THEN auto-pass behaves exactly as it does without auto mode.

### Requirement: Generated surfaces document auto-mode semantics
REQ-008: The generated command/skill surfaces (toolgen) and the README MUST
document `execution.auto` and the `slipway run --auto/--no-auto` flags, and MUST
teach hosts that under auto a standing-authorization boundary may be crossed on
prior authorization through pure-pacing pauses while the retained human stops
(sensitive/guardrail confirmations, intake Approved Summary, decision
checkpoints, and every evidence gate) still require an explicit stop. The toolgen
command-description contract test MUST stay green.

#### Scenario: Run surface advertises the auto flags
GIVEN the generated `slipway run` command surface
WHEN the command arguments and description are rendered
THEN they include `--auto` and `--no-auto`
AND the command-description contract test passes.

#### Scenario: README documents auto mode and its red lines
GIVEN the README
WHEN it describes execution configuration
THEN it documents `execution.auto` and the per-run override flags
AND it states that sensitive/guardrail confirmations, the intake Approved
Summary, decision checkpoints, and evidence gates are never auto-advanced.
