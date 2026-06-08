# Concerns

Re-authored for change `fix-pending-approach-reported-as-locked-decision`
(issue #140); replaces the stale issue-#114 map.

- Architectural pressure points:
  - `locked_decisions` is a public `next --json` contract field; its meaning
    ("the plan gate has locked this in") must stay honest. Adding
    `pending_decisions` is additive but still a contract change to review.
  - The lock signal must come from the lifecycle (`G_plan` gate), not from
    expanding the placeholder text matcher — text heuristics were explicitly
    rejected and would re-introduce the same false-positive class.
- Brittle areas:
  - `deriveConfirmationRequirement` runs AFTER skill_constraints assembly; do
    not try to reuse `view.ConfirmationRequirement` inside
    `buildSkillConstraints` (it is not populated yet). Use the readiness/G_plan
    status which is available earlier.
  - `cloneSkillConstraints` must copy any new field or handoff payloads silently
    drop it.
- Migration traps:
  - Editing generated `.claude/`/`.codex/` skill copies by hand drifts from the
    template source of truth — change the `spec-compliance-review` TEMPLATE and
    regenerate via toolgen.
- Fail-closed requirement:
  - A decision still pending fresh confirmation (G_plan not approved) must NEVER
    appear in `locked_decisions`. The confirmed path (G_plan approved) must keep
    populating `locked_decisions` so spec-compliance fidelity checks still work.
- Recheck routing:
  - Run focused `cmd/` tests for
    `TestSkillConstraintsPendingDecisionsBeforePlanLock`,
    `TestSkillConstraintsLockedDecisionsAfterPlanLock`,
    `TestBuildSkillConstraintsLockedVsPending`, and
    `TestPlanLockedFromGates`; run `internal/tmpl` / `internal/toolgen`
    generated-surface tests, then full `go build/vet/test ./...`.
- Notes / source references:
  - `cmd/next.go`, `cmd/next_skill.go`, `internal/engine/gate/gate.go`,
    `cmd/next_handoff.go`, `internal/tmpl/templates/skills/` (spec-compliance-review).
