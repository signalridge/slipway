# Research

## Alternatives Considered

### Architecture
- Affected modules / files:
  - `internal/engine/artifact/manager.go` — `ParseDecisionLockedDecisions`
    (~L443) parses decision.md "Selected Approach" / "Selected Direction" and
    gates ONLY on `LooksLikeTemplatePlaceholder` (~L474). Author-written prose
    marked "Pending explicit user confirmation" is not a placeholder, so it
    leaks into locked decisions.
  - `cmd/next.go` — `skillConstraints` struct (~L119-131, json tags); the JSON
    contract surface; `deriveConfirmationRequirement` (~L609) and
    `confirmationHardStop`/`confirmationCommandRequired` helpers.
  - `cmd/next_skill.go` — `buildSkillConstraints` (~L20-42) and
    `parseLockedDecisions` (~L59-74) wrapper that calls the artifact parser.
  - `cmd/next_skill_view.go` — `assembleSkillViewWithOptions` (~L78) calls
    `buildSkillConstraints` (~L332); this is the threading point.
  - `cmd/next_handoff.go` — `cloneSkillConstraints` (~L253) deep-copies the
    struct for handoff payloads; must copy any new field.
  - `internal/engine/gate/gate.go` — `G_plan` evaluation (~L66-86) =
    bundleReady + planAuditPass + DecisionContractBlockers. This is the
    lifecycle "plan locked" gate and the chosen lock signal.
- Dependency chains:
  - `next`/`run` RunE → `buildNextView` → `buildNextContextByMode` (loads
    Change + readiness + GateEvaluations incl. G_plan) → `assembleSkillView` →
    `assembleSkillViewWithOptions` → `buildSkillConstraints` →
    `parseLockedDecisions` → `artifact.ParseDecisionLockedDecisions`.
  - `view.ConfirmationRequirement` is derived in `finalize()` AFTER
    `assembleSkillView` returns — so it is NOT available at skill_constraints
    assembly time. The readiness gate evaluations ARE computed earlier and can
    be threaded in.
- Blast radius: `slipway next`/`run` JSON `skill_constraints`. Read by the
  `spec-compliance-review` host (Decision Fidelity Check) and cloned for
  handoff payloads. No engine gate weakening.
- Constraints / invariants:
  - `internal/tmpl/templates/skills/` is the SOURCE OF TRUTH for generated
    skills; `.claude`/`.codex` copies must be regenerated via toolgen, never
    hand-edited (per ARCHITECTURE codebase map).
  - `locked_decisions` semantics must mean "the plan gate has locked this in",
    not "the text is non-placeholder".

### Patterns
- Existing conventions:
  - `skillConstraints` uses `json:"...,omitempty"` string-slice fields
    (`locked_decisions`); a new `pending_decisions []string
    json:"pending_decisions,omitempty"` field mirrors that idiom exactly.
  - Gate status is surfaced as `gate_status.G_plan.status` (`approved` /
    `blocked` / ...) — see `slipway status --json`.
- Reusable abstractions:
  - `readiness` / `GateEvaluations` already carry G_plan status; thread the
    G_plan status (or a derived `planLocked bool`) into
    `assembleSkillViewWithOptions` → `buildSkillConstraints`.
  - `ParseDecisionLockedDecisions` already returns the parsed (non-placeholder)
    decisions; reuse its output and split it into locked vs pending based on the
    gate signal rather than changing the parser's placeholder logic.
- Convention deviations: none required. No new text-marker heuristics are
  added (explicitly out of scope).

### Risks
- Technical risks:
  - [low] Threading gate status into skill assembly: a mechanical signature
    change; must update all call sites of `assembleSkillView*`.
  - [low] Regression on already-locked decisions: once G_plan is `approved`
    (S2+/review), locked_decisions must still populate so
    `spec-compliance-review` fidelity check keeps working — covered by tests.
  - [medium] Consumer drift: `spec-compliance-review` SKILL template must learn
    that `pending_decisions` is advisory (do NOT enforce fidelity on pending),
    and be regenerated via toolgen so `.claude`/`.codex` stay in sync.
- Guardrail domains: external API / contract surface (public `next --json`).
  Review as an external contract change. Fail-closed: a decision still pending
  fresh confirmation must never appear as locked.
- Reversibility: fully reversible (additive field + a conditional); no data
  migration, no irreversible ops.

### Test Strategy
- Existing coverage:
  - `cmd/next_skill_constraints_test.go` — `TestParseLockedDecisions`,
    `TestSkillConstraintsLockedDecisionsFromDecision` (asserts populated).
  - `internal/engine/artifact/*` — placeholder/parse tests.
  - There is existing assertion that unconfirmed/placeholder decisions yield nil
    locked decisions; the new gate signal must not break the confirmed path.
- Infrastructure needs: none new; reuse table-driven cmd tests + an e2e in
  `cmd/cli_e2e_test.go` style that drives a change to a pre-confirm vs
  post-G_plan-approved state and asserts the field split.
- Verification approach per acceptance signal:
  - Pending scenario (G_plan not approved) → assert recommended approach is in
    `pending_decisions` and absent from `locked_decisions`.
  - Confirmed scenario (G_plan approved) → assert it is in `locked_decisions`
    and absent from `pending_decisions`.
  - Placeholder/no decision → both empty.
  - Regenerated `spec-compliance-review` surface contains the pending advisory
    (template/toolgen test).

### Options
- Approach A (G_plan gate status) — locked iff `G_plan` is `approved`; otherwise
  the parsed non-placeholder decisions go to `pending_decisions`. Thread G_plan
  status from readiness into `buildSkillConstraints`. Tradeoff: a small
  signature/threading change; gain: semantically precise, ties to the real
  "plan locked" gate.
- Approach B (lifecycle stage) — pending while in S0/S1, locked at S2+. No gate
  threading needed. Tradeoff: coarser; cannot express "G_plan approved but still
  S1" and conflates stage with plan-lock.
- Approach C (reuse confirmation_requirement) — pending while a fresh hard_stop
  confirmation is required. Tradeoff: that signal is derived AFTER skill
  assembly (ordering rework) and is a broad per-handoff signal, not
  decision-specific.
- Selected: **Approach A — G_plan gate status** (user-selected). Rationale:
  G_plan is the lifecycle authority for "the plan/decision is locked in"; it is
  computed before skill assembly (so no ordering rework), and it keeps the
  parser's placeholder logic untouched while making locked-vs-pending honest.

## Unknowns
- Resolved: Authoritative confirmation signal reachable at skill_constraints
  assembly time -> `G_plan` gate status from `readiness.GateEvaluations`
  (computed in `buildNextContextByMode`, before `assembleSkillView`).
  `view.ConfirmationRequirement` is NOT usable (derived later in `finalize()`).
- Resolved: Does `buildSkillConstraints` already have the gate state? -> No; it
  receives `(root, def, governedChange)`. The G_plan status must be threaded
  from `buildNextView` through `assembleSkillView(WithOptions)` into
  `buildSkillConstraints`.
- Resolved: New JSON field name/shape + consumers -> `pending_decisions
  []string json:"pending_decisions,omitempty"` on `skillConstraints`
  (mirrors `locked_decisions`). Consumers to update: `cloneSkillConstraints`
  (`cmd/next_handoff.go`), the `spec-compliance-review` skill TEMPLATE
  (`internal/tmpl/templates/skills/.../spec-compliance-review` + toolgen
  regeneration), and tests.
- Remaining: None.

## Assumptions
- `G_plan.status == "approved"` is a faithful proxy for "decision is locked in"
  - Evidence: `internal/engine/gate/gate.go` G_plan = bundleReady +
    planAuditPass + DecisionContractBlockers; once approved the plan bundle
    (including decision.md) is the locked authority. Confirmed by
    `slipway status --json` `gate_status.G_plan`.
- The recommended-but-pending approach is still worth surfacing (not dropped)
  - Evidence: issue #140 "JSON field should clarify the pending/unlocked state";
    downstream skills benefit from seeing the recommendation while knowing it is
    unconfirmed.
- Generated skill surfaces must be edited at the template, not the copy
  - Evidence: `artifacts/codebase/ARCHITECTURE.md` and repo `CLAUDE.md` change
    discipline ("keep code, generated skills, docs aligned as one product
    surface").

## Canonical References
- `internal/engine/artifact/manager.go` (ParseDecisionLockedDecisions, LooksLikeTemplatePlaceholder)
- `cmd/next.go` (skillConstraints struct, deriveConfirmationRequirement)
- `cmd/next_skill.go` (buildSkillConstraints, parseLockedDecisions)
- `cmd/next_skill_view.go` (assembleSkillViewWithOptions, buildSkillConstraints call site)
- `cmd/next_handoff.go` (cloneSkillConstraints)
- `internal/engine/gate/gate.go` (G_plan evaluation)
- `cmd/next_skill_constraints_test.go` (existing locked-decisions tests)
- `internal/tmpl/templates/skills/` (spec-compliance-review template; generated-surface source of truth)
