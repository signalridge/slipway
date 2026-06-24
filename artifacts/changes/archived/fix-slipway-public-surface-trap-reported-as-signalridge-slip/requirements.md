# Requirements

## Requirements

### Requirement: Cache-unreadable failures must point at the engine-owned wave-plan cache, not tasks.md
REQ-001: When the persistable wave-plan cache `wave-plan.yaml` cannot be parsed or validated (for example it carries view-only / unsupported fields such as `wave_count` or `advisories`, or is otherwise corrupt), the command-surface error built in `loadAuthoritativeWaveExecution` (`cmd/common.go`) MUST surface a reason code and remediation that name the engine-owned `wave-plan.yaml` cache and the public regenerate path (`slipway repair` to rebuild it from `tasks.md`, then `slipway run` to refresh affected execution evidence), state that the cache must not be hand-edited, and MUST NOT instruct the user to update `tasks.md`.

#### Scenario: wave-plan.yaml carries view-only fields
GIVEN an active change whose `wave-plan.yaml` contains the view-only fields `wave_count` and `advisories`
WHEN a command path loads authoritative wave execution (e.g. `slipway next --json --diagnostics`)
THEN the returned error remediation names the engine-owned wave-plan cache and a regenerate command (`slipway repair`)
AND the remediation does not tell the user to update `tasks.md`
AND the cache is described as engine-owned / not hand-editable.

### Requirement: Genuine tasks.md-derivation failures keep tasks.md-oriented guidance
REQ-002: When the current wave plan cannot be derived because `tasks.md` itself is unschedulable (parse/dependency/scope failure during derivation), the error MUST continue to direct the user to fix `tasks.md`. The cache-oriented remediation from REQ-001 MUST be reserved for cache parse/validation failures, distinguished by a typed/sentinel error rather than fragile string matching.

#### Scenario: tasks.md cannot be converted into a wave plan
GIVEN an active change whose `tasks.md` cannot be converted into a schedulable wave plan
WHEN a command path attempts to derive the current wave plan
THEN the returned error remediation directs the user to update `tasks.md`
AND it is not the cache-oriented remediation defined in REQ-001.

### Requirement: View-only projection fields stay out of the persisted cache and are documented as diagnostic-only
REQ-003: The diagnostic projection `input_context.wave_plan` (`cmd/next.go` `wavePlanView`) MUST be documented as a diagnostic-only, non-persistable projection of `slipway next --json`, distinct from the persistable `wave-plan.yaml` schema (`model.WavePlan`). Its view-only fields (`wave_count`, `advisories`) MUST never be written into the persisted `wave-plan.yaml` cache. No existing `input_context.wave_plan` JSON field name or structure is changed (non-breaking).

#### Scenario: materialized wave plan excludes view-only fields
GIVEN a materialized `wave-plan.yaml` produced by the engine
WHEN the persisted cache is inspected
THEN it contains no `wave_count` or `advisories` field
AND the `wavePlanView` projection is annotated/documented as diagnostic-only and non-persistable.

### Requirement: The wave-plan cache stays fail-closed
REQ-004: The strict `KnownFields(true)` parsing of `wave-plan.yaml` MUST be preserved and `model.WavePlan` MUST NOT be grown to accept the view-only fields. The fix improves diagnosis and remediation only; it does not relax the cache schema or its fail-closed behavior.

#### Scenario: unknown field is still rejected
GIVEN a `wave-plan.yaml` containing an unsupported field
WHEN the loader parses it
THEN parsing still fails closed (the unknown field is rejected)
AND the failure is reported as a cache-unreadable condition (REQ-001) rather than silently accepted.
