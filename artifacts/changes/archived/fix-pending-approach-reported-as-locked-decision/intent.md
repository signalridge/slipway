# Intent

## Summary
Fix issue #140: `slipway next --json` reports a recommended-but-unconfirmed
decision (a `decision.md` "Selected Approach" labeled "Pending explicit user
confirmation") inside `skill_constraints.locked_decisions`, even while the same
response sets `confirmation_requirement.required: true` with
`boundary: hard_stop`. A pending decision must not be presented as locked, and
the JSON must make the pending/unlocked state explicit.

## Complexity Assessment
complex
<!-- Rationale -->
Root cause is not a one-line text fix: `ParseDecisionLockedDecisions`
(`internal/engine/artifact/manager.go`) currently treats any non-template-
placeholder "Selected Approach"/"Selected Direction" text as a locked decision.
Author-written prose marked pending is real content, so a placeholder-phrase
list cannot reliably distinguish pending from locked. The chosen fix ties
"locked" to the governed lifecycle confirmation state and adds a new JSON
surface, which touches the public `next --json` contract and the data flow
between `cmd/next.go` (confirmation derivation) and `cmd/next_skill.go`
(skill_constraints assembly).

## Guardrail Domains
External API / contract surface: this changes the public `slipway next --json`
output (`skill_constraints`). Must be reviewed as an external contract change.

## In Scope
- `internal/engine/artifact/manager.go` — `ParseDecisionLockedDecisions` and how
  "Selected Approach"/"Selected Direction" become locked decisions.
- `cmd/next_skill.go` — `buildSkillConstraints` / `parseLockedDecisions`: gate
  `locked_decisions` on confirmation state; add the new explicit pending field
  to the `skill_constraints` struct.
- `cmd/next.go` — reuse the lifecycle confirmation signal
  (`deriveConfirmationRequirement` / `confirmationRequirement`) as the authority
  for "confirmed vs pending"; thread it into skill_constraints assembly if not
  already available.
- Tests in `cmd/` and `internal/engine/artifact/` covering the pending→locked
  transition and the new JSON field.
- Generated `next --json` contract docs / command reference where the field set
  is documented.

## Out of Scope
- The unrelated active change `fix-136-...` and any other active governed change
  or worktree — left untouched.
- Changing *how* a decision gets confirmed (the confirmation/approval flow
  itself); only *how* locked-vs-pending is reported is in scope.
- Adding new fragile text-marker heuristics for pending detection (explicitly
  rejected in favor of the lifecycle-state signal).

## Constraints
- Determination must be driven by the governed lifecycle confirmation state, not
  by expanding the placeholder-phrase text matcher.
- Public JSON contract change: review `next --json` as an external contract;
  keep `locked_decisions` semantics ("already confirmed/locked") honest.
- Use the current worktree's Slipway CLI (dev build) as the source of truth.
- Fail closed: a decision still pending fresh confirmation must never appear as
  locked.

## Acceptance Signals
- Repro: with a `decision.md` "Selected Approach" that is recommended but still
  pending confirmation (response carries `confirmation_requirement.required:true`,
  `boundary:hard_stop`), `slipway next --json` does NOT list that approach under
  `skill_constraints.locked_decisions`.
- The same response surfaces the recommended approach under a new explicit
  pending field in `skill_constraints`, clearly marked unconfirmed.
- After the decision is confirmed/locked through the governed flow, it appears
  in `locked_decisions` and no longer in the pending field.
- New unit/e2e tests assert both the exclusion and the new field, and the
  pending→locked transition; `go build/vet/test ./...` green.

## Open Questions
- [x] Authoritative lifecycle signal → `G_plan` gate status from
  `readiness.GateEvaluations` (computed before skill assembly).
  `confirmationRequirement` is derived later (`finalize()`) and is unusable.
- [x] Does `buildSkillConstraints` have the gate state? → No; thread G_plan
  status from `buildNextView` through `assembleSkillView` into
  `buildSkillConstraints`.
- [x] New pending field → `pending_decisions []string` on `skillConstraints`;
  consumers: `cloneSkillConstraints`, the `spec-compliance-review` skill
  template (regenerate via toolgen), and tests. (See research.md.)

## Deferred Ideas
<!-- none -->

## Approved Summary
Confirmed 2026-06-08T14:38:13Z.

Fix issue #140 so `slipway next --json` stops presenting a recommended-but-
unconfirmed decision as locked. Determine "locked vs pending" from the governed
lifecycle confirmation state (not a placeholder text matcher): a decision enters
`skill_constraints.locked_decisions` only once it has been confirmed through the
flow; while the same response still requires fresh confirmation
(`confirmation_requirement.required:true`, `boundary:hard_stop`), the recommended
approach is excluded from `locked_decisions` and surfaced under a new explicit
pending field so downstream skills still see it but know it is unconfirmed.

In scope: decision parsing in `internal/engine/artifact/manager.go`,
skill_constraints assembly + new pending field in `cmd/next_skill.go`,
confirmation-signal reuse in `cmd/next.go`, tests, and `next --json` contract
docs. Out of scope: the unrelated `fix-136` change, the confirmation flow
itself, and new text-marker heuristics.

Primary acceptance signal: in a pending-confirmation scenario the recommended
approach is absent from `locked_decisions` and present in the new pending field;
after confirmation the two reverse; `go build/vet/test ./...` green.
