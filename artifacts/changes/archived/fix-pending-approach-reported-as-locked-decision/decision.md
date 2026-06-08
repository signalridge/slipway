# Decision

## Alternatives Considered
Three ways to determine "locked vs pending" for a `decision.md` Selected
Approach (the high-level direction — bind to lifecycle confirmation state plus a
new explicit pending field — was confirmed with the user during intake):

- **Approach A — `G_plan` gate status (SELECTED).** A decision is locked iff the
  `G_plan` gate is `approved`; otherwise the parsed non-placeholder decisions
  are reported as pending. Tradeoff: requires threading the gate status from
  readiness into skill_constraints assembly; gain: semantically precise, ties to
  the real lifecycle "plan locked" gate, no ordering rework, parser untouched.
- **Approach B — lifecycle stage.** Pending while in `S0_INTAKE`/`S1_PLAN`,
  locked at `S2_EXECUTE`+. Tradeoff: no gate threading, but coarser — conflates
  stage with plan-lock and cannot express "G_plan approved but still S1".
- **Approach C — reuse `confirmation_requirement`.** Pending while a fresh
  `hard_stop` confirmation is required. Tradeoff: that signal is derived AFTER
  skill_constraints assembly (`finalize()`), forcing an ordering rework, and is
  a broad per-handoff signal, not decision-specific.

Evidence: data-flow trace in `research.md` (`## Alternatives Considered`),
`cmd/next.go` (`deriveConfirmationRequirement` ordering), `internal/engine/gate/
gate.go` (`G_plan`).

## Selected Approach
Approach A — gate `locked_decisions` on the `G_plan` gate state and add an
explicit `pending_decisions` field.

- Add `PendingDecisions []string \`json:"pending_decisions,omitempty"\`` to the
  `skillConstraints` struct in `cmd/next.go`.
- Thread the `G_plan` gate status (a derived `planLocked bool`) from
  `buildNextView`/readiness through `assembleSkillView(WithOptions)` into
  `buildSkillConstraints` (`cmd/next_skill.go` / `cmd/next_skill_view.go`).
- Keep `ParseDecisionLockedDecisions` and `LooksLikeTemplatePlaceholder`
  unchanged (placeholder filtering still removes scaffold text). Take the parser
  output and route it: when `planLocked` → `locked_decisions`; otherwise →
  `pending_decisions`.
- Preserve `pending_decisions` in `cloneSkillConstraints`
  (`cmd/next_handoff.go`).
- Update the `spec-compliance-review` template so Decision Fidelity enforces
  only `locked_decisions` and treats `pending_decisions` as advisory; regenerate
  generated surfaces via toolgen.

Why: `G_plan` is the lifecycle authority for "the plan/decision is locked in",
is computed before skill assembly (no reordering), keeps the parser honest, and
makes `locked_decisions` mean what it says while preserving the recommendation
under a clearly-unconfirmed field.

## Interfaces and Data Flow
- Public JSON contract (`slipway next --json`): `skill_constraints` gains
  `pending_decisions` (string list, omitempty). `locked_decisions` semantics
  tighten to "G_plan-approved decisions only".
- Internal: `buildSkillConstraints(root, def, change)` gains a `planLocked`
  (gate-state) parameter; `assembleSkillView`/`assembleSkillViewWithOptions`
  signatures extend to pass it; `cloneSkillConstraints` copies the new slice.
- Data flow: readiness `GateEvaluations[G_plan].status == approved` → `planLocked`
  → split `ParseDecisionLockedDecisions` output into locked vs pending.
- Consumer surface: generated `spec-compliance-review` SKILL text (template
  source `internal/tmpl/templates/skills/...`, regenerated copies).

## Rollout and Rollback
- Rollout: single change; additive JSON field + a conditional split + consumer
  guidance regeneration. No data migration, no kernel half-states between waves.
- Verification command: `go build ./... && go vet ./... && go test ./...`, plus
  the new locked-vs-pending unit/e2e tests and the regenerated-surface template
  test.
- Rollback: revert the change (or drop the `pending_decisions` field and the
  gate-threading); the field is `omitempty` and additive, so removing it
  restores prior output. Rollback verification: `go test ./...` on the reverted
  tree.

## Risk
- [low] Threading the gate state changes `assembleSkillView*` signatures — must
  update all call sites; caught by compilation + tests.
- [medium] Regression on the confirmed path: once `G_plan` is approved,
  `locked_decisions` must still populate so `spec-compliance-review` fidelity
  checks keep working. Mitigation: explicit test for the post-approval case.
- [medium] Consumer drift: the `spec-compliance-review` guidance must change at
  the TEMPLATE and be regenerated; hand-editing the `.claude`/`.codex` copy
  would drift. Mitigation: edit template + run toolgen + template test asserts
  the new advisory text.
- [low] Contract surface: `next --json` is external; the change is additive
  (new omitempty field) and tightens an over-broad field — reviewed as a
  contract change at S3.
