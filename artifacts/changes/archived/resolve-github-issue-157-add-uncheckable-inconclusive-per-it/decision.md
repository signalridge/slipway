# Decision

## Alternatives Considered

### Option 1: Extend authored skill-output contract
Update `internal/tmpl/templates/skills/spec-trace/SKILL.md`,
`internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl`, and targeted
spec-compliance-review guidance. Add focused template regressions in
`internal/tmpl/templates_test.go`.

Tradeoffs: directly satisfies the remaining Issue #157 gap with low code blast
radius; relies on the skill-output contract rather than runtime parsing.

### Option 2: Add runtime parser and validator for trace rows
Introduce engine-owned parsing for spec-trace coverage matrices and enforce row
statuses in validation.

Tradeoffs: stronger machine enforcement, but it widens the change into runtime
schema behavior and conflicts with the issue instruction to avoid new engine
prose heuristics.

### Option 3: Only add prose to spec-compliance-review
Tell spec-compliance-review not to silently accept uncertain mappings without
changing spec-trace row vocabulary.

Tradeoffs: smallest edit, but it leaves the first-class per-item record gap in
spec-trace unresolved.

## Selected Approach

Select Option 1. The issue's remaining real scope is the missing per-item status
bucket and coverage accounting, and the current architecture already places that
contract in spec-trace and spec-compliance-review authored templates. This keeps
the change narrow while still making "could not check" auditable.

## Interfaces and Data Flow

- Interface changed: markdown contract exposed by `slipway-spec-trace` and used
  by spec-compliance-review/review/validate surfaces.
- Data flow:
  - authored template files -> `internal/tmpl` rendering -> exported skill
    prompts -> reviewer-authored verification reports.
- Runtime data structures: none changed.
- External service contracts: none changed.

## Rollout and Rollback

- Rollout:
  - Add RED template test for the Issue #157 contract.
  - Update templates to satisfy it.
  - Run targeted and broad Go verification, then Slipway governance gates.
- Rollback:
  - Revert edits to the three template/test files and rerun
    `go test ./internal/tmpl`.
  - Because no runtime state or schema is migrated, rollback is a normal git
    revert.

## Risk

- Medium governance risk if ambiguous/uncheckable rows are presented as benign
  skips. Mitigation: explicit reason and coverage-gap accounting plus
  spec-compliance pass-blocking guidance.
- Low compatibility risk because existing `covered`, `skipped`, and `drift`
  statuses remain valid.
- Low implementation risk because no engine runtime schema or generated copies
  are edited directly.
