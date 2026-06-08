# Architecture

Re-authored for change `fix-pending-approach-reported-as-locked-decision`
(issue #140). The prior map described issue #114 (skill-template thinning) and
was out of scope; this scopes to the `slipway next --json` skill_constraints
path.

- Module responsibilities:
  - `cmd/next.go` — owns the `next`/`run` command, the `skillConstraints` JSON
    struct (the public `next --json` contract), and `deriveConfirmationRequirement`.
  - `cmd/next_skill.go` — `buildSkillConstraints` / `parseDecisionItems`:
    parses selected decision text and routes it into
    `skill_constraints.locked_decisions` or
    `skill_constraints.pending_decisions` according to the G_plan gate.
  - `cmd/next_skill_view.go` — `assembleSkillViewWithOptions` resolves the next
    skill and calls `buildSkillConstraints` (the threading point for gate state).
  - `cmd/next_handoff.go` — `cloneSkillConstraints` deep-copies the struct for
    handoff payloads.
  - `internal/engine/artifact/manager.go` — `ParseDecisionLockedDecisions` +
    `LooksLikeTemplatePlaceholder`: parse decision.md sections; placeholder
    detection (NOT confirmation detection).
  - `internal/engine/gate/gate.go` — `G_plan` gate evaluation; the lifecycle
    authority for "the plan/decision is locked in" (the chosen lock signal).
  - `internal/tmpl/templates/skills/` — SOURCE OF TRUTH for generated skill
    surfaces (e.g. spec-compliance-review). `.claude`/`.codex` copies are
    regenerated via `internal/toolgen`; never hand-edited.
- Dependency flow:
  - `next`/`run` RunE → `buildNextView` → `buildNextContextByMode` (loads Change
    + readiness + `GateEvaluations` incl. G_plan) → `assembleSkillView` →
    `assembleSkillViewWithOptions` → `buildSkillConstraints` →
    `parseDecisionItems` → `artifact.ParseDecisionLockedDecisions`; the
    returned items are locked only when G_plan is approved and pending otherwise.
  - Ordering: `view.ConfirmationRequirement` is derived in `finalize()` AFTER
    skill assembly; the readiness gate evaluations are computed BEFORE it, so
    G_plan status (not confirmation_requirement) is the usable signal.
- Coupling hotspots:
  - `skill_constraints.locked_decisions` is read by the `spec-compliance-review`
    host (Decision Fidelity Check); `pending_decisions` is advisory context.
    Both fields are cloned in handoff payloads and must stay in sync.
- Current change blast radius:
  - `skillConstraints` struct (+`pending_decisions`), `buildSkillConstraints`
    signature/logic, the assembleSkillView threading, `cloneSkillConstraints`,
    the spec-compliance-review template + toolgen regeneration, and tests.
  - No engine gate weakening; decision parser placeholder logic unchanged.
- Notes / source references:
  - `cmd/next.go`, `cmd/next_skill.go`, `cmd/next_skill_view.go`,
    `cmd/next_handoff.go`, `internal/engine/artifact/manager.go`,
    `internal/engine/gate/gate.go`,
    `internal/tmpl/templates/skills/` (spec-compliance-review).
