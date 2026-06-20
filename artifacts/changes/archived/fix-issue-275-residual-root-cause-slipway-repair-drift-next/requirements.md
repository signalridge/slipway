# Requirements

## Requirements

### Requirement: Repair drift guidance routes tasks.md parse failures to fixing tasks.md
REQ-001: `slipway repair` MUST, for an unrepaired-drift finding whose reason
indicates a tasks.md parse failure — an unknown/unsupported task metadata key
(`internal/engine/wave/parse.go` "uses unknown metadata key") or a wave-plan
derivation/load failure (`wave_plan_load_failed`) — emit a `next_action` that
directs the operator to edit tasks.md to fix or remove the offending content and
then re-run `slipway repair` / `slipway validate`. It MUST NOT fall through to
the generic default `next_action` ("run `slipway run` to repair the current
lifecycle evidence and continue alignment").

#### Scenario: Unknown metadata key in tasks.md at S2
GIVEN an active S2_IMPLEMENT change with a materialized wave plan and recorded
execution evidence
AND its governed tasks.md is edited to carry an unknown metadata key (e.g.
`scope_amendment`)
WHEN the operator runs `slipway repair --json`
THEN the resulting `unrepaired_drift[]` finding for that parse failure has a
`next_action` that instructs editing tasks.md to remove/fix the unsupported
metadata key and re-running `slipway repair` / `slipway validate`
AND that `next_action` contains no "run `slipway run`" instruction.

### Requirement: Repair stays fail-closed and never mutates governed tasks.md
REQ-002: `slipway repair` MUST treat the operator's governed tasks.md as
read-only for this drift class. It MUST NOT auto-remove or rewrite the offending
metadata key, MUST NOT acquire a force/bypass/auto-rewrite path for governed
artifacts, and MUST surface the parse-failure drift as operator-actionable
guidance only.

#### Scenario: Repair leaves the offending tasks.md unchanged
GIVEN an S2 change whose governed tasks.md carries an unknown metadata key
WHEN the operator runs `slipway repair`
THEN tasks.md is left byte-for-byte unchanged
AND the unknown metadata key still surfaces as an unrepaired-drift finding
rather than being silently repaired.

### Requirement: Parse-failure remediation is consistent across repair, validate, run, and next
REQ-003: For tasks.md parse-failure drift, the remediation guidance surfaced by
`slipway repair`, `slipway validate`, `slipway run`, and `slipway next` MUST be
mutually consistent: every surface MUST point the operator at fixing tasks.md and
none MUST route to "run `slipway run`" or another dead-end for this drift class.
Where the already-correct sibling surfaces (`cmd/common.go` `wave_plan_load_failed`
remediation; validate's tasks-checklist invalid-format remediation) diverge in
wording, they MUST be aligned to read as one product surface for this drift.

#### Scenario: All four surfaces agree on fixing tasks.md
GIVEN an active S2 change whose governed tasks.md carries an unknown metadata key
WHEN the operator consults `slipway repair --json`, `slipway validate --json`,
`slipway run --json`, and `slipway next --json`
THEN each surface's guidance directs the operator to edit/fix tasks.md before
continuing
AND none of the four surfaces instructs the operator to "run `slipway run`" as
the remedy for this drift.
