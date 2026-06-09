# Intent

## Summary
Defer assurance.md creation to S3_REVIEW so it is no longer scaffolded early at S1_PLAN/bundle, aligning it with the #119 deferred-creation design (the lone remaining exception). Add assurance.md to deferredToSkillAuthoring; remove the plan-audit-digest exclusion special-case; the review/verify skill authors it at S3 where AssuranceContractBlockers already enforces it. Verify light preset and slipway preset re-scaffold paths.

While dogfooding this change, two adjacent governed surfaces were found inconsistent with the deferral/scope model and are aligned here: (1) the `slipway repair` / doctor bundle-consistency diagnostic, which after deferral would warn on every standard/strict change pre-S3 about a by-design-absent assurance.md; and (2) the out-of-scope drift recovery path, where `slipway pivot --rescope` was reachable only from S2_EXECUTE — so a legitimate review-time out-of-scope edit (which reopens to S3_REVIEW for stale review evidence) could not reach the documented recovery, and the recovery guidance misdescribed rescope as a non-destructive tasks.md edit.

## Complexity Assessment
complex
<!-- Rationale: touches the engine artifact lifecycle (scaffolding/deferral), evidence-digest inputs, the public instructions guidance surface, two generated host-skill templates, and end-user docs. Multiple coupled surfaces must stay aligned and fail-closed; not a one-line edit. -->

## Guardrail Domains
<!-- none detected — internal governance-engine lifecycle change, no auth/credentials/PII/financial/schema-migration/irreversible/external-API surface. -->

## In Scope
- `internal/engine/artifact/manager.go`: add `"assurance.md"` to `deferredToSkillAuthoring` so the engine no longer scaffolds it at S1_PLAN/bundle (a missing required assurance.md is then fail-closed, not a placeholder body).
- `internal/engine/progression/evidence_digests.go`: remove the now-unnecessary `assurance.md` exclusion rationale comment in `addPlanningArtifactInputs` (assurance.md simply does not exist at plan-audit time after deferral).
- `cmd/instructions.go`: rewrite the `assurance` guidance string so it stops claiming "the engine may create the scaffold" and instead states the engine defers the body (author from the template), matching the requirements/decision/tasks/research phrasing.
- Generated host-skill templates that describe assurance timing: `internal/tmpl/templates/skills/plan-audit/SKILL.md` and `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl` — update any wording that assumes an engine-created early scaffold.
- End-user docs referencing assurance scaffolding/timing: `docs/workflow.md`, `docs/design.md`, `docs/commands.md`, `docs/operator-guide.md`, `docs/index.md` (text only; review for stale "scaffolded at S1" claims).
- Tests: add/adjust unit tests proving assurance.md is NOT scaffolded by `ScaffoldGovernedBundleForChange` on standard/strict preset, that `slipway preset` re-scaffold does not create it, and that the light preset path stays a no-op.
- `internal/state/repair.go`: `DiagnoseBundleConsistency` must stay silent on a deferred (absent) `assurance.md` before S3_REVIEW (it is the expected deferred state, not a partial-write inconsistency) while keeping the S3+/done "missing assurance" consistency error.
- `internal/engine/gate/gate.go` + `cmd/pivot_validation.go`: allow `slipway pivot --rescope` from S2_EXECUTE/S3_REVIEW/S4_VERIFY (it resets to S0_INTAKE regardless), keeping it rejected before execution, so out-of-scope drift recovery is reachable after a stale-evidence reopen to S3_REVIEW.
- `internal/model/recovery.go` + `internal/engine/progression/readiness.go`: rewrite the `scope_contract_drift` recovery guidance to lead with the non-destructive amend-`tasks.md`-`target_files` path and to describe `pivot --rescope` honestly as a full re-plan that resets to S0_INTAKE.
- `docs/commands.md`: update the documented valid rescope states and the non-destructive scope-drift recovery path.

## Out of Scope
- The placeholder/scaffold floor (`assuranceSectionLooksScaffold`, issue #47): retained as a defense-in-depth backstop, demoted from primary guard — NOT removed.
- The #47 scaffold-placeholder floor: retained as a defense-in-depth backstop, not removed.
- The assurance enforcement window: missing/empty/scaffold-only assurance remains fail-closed at S3_REVIEW and later, now explicitly including DONE and explicit unknown lifecycle states.
- Converting any other artifact's creation timing, or any change to requirements/decision/tasks/research deferral.
- The embedded `assurance.md` template content itself (still the source for `slipway instructions assurance`).

## Constraints
- Closeout must remain fail-closed: missing, empty, or scaffold-only assurance bodies must still be rejected before `done`.
- Keep code, generated skills, and docs aligned as one product surface (CLAUDE.md).
- Smallest clean change; remove the obsolete early-scaffold behavior rather than preserving compatibility for it.

## Acceptance Signals
- `assurance.md` does not exist on disk before `S3_REVIEW` on standard/strict preset (unit test on `ScaffoldGovernedBundleForChange`; end-to-end dogfood on this standard-preset change).
- The review/verify host authors assurance.md at S3 from `slipway instructions assurance`; advancing past S3 stays blocked (`assurance_contract_missing` / per-section placeholder) until it is authored; `done`-readiness remains fail-closed for missing/empty/scaffold bodies.
- The plan-audit digest no longer carries the `assurance.md` exclusion comment/branch and behaves correctly (assurance edits at S3+ do not retroactively stale the plan-audit digest because the file is simply absent at plan-audit time).
- `slipway preset` re-scaffold and the `light` preset path show no regression (assurance.md not created early in either).
- `DiagnoseBundleConsistency` emits no error/warning for a deferred-absent assurance.md before S3_REVIEW, and still reports it as a consistency error at S3_REVIEW/S4_VERIFY/done.
- `slipway pivot --rescope` is accepted from S2_EXECUTE/S3_REVIEW/S4_VERIFY (resetting to S0_INTAKE) and rejected with `rescope_state_invalid` from S0_INTAKE/S1_PLAN.
- The `scope_contract_drift` recovery guidance leads with the non-destructive amend-`tasks.md`-`target_files` path and describes `pivot --rescope` as a reset to S0_INTAKE; `docs/commands.md` matches.
- `go build ./... && go vet ./... && go test ./...` green; `gofmt` clean.

## Open Questions
None.

## Deferred Ideas
<!-- none -->

## Approved Summary
Defer `assurance.md` creation from S1_PLAN/bundle to S3_REVIEW, where the review/verify host authors it from scratch via `slipway instructions assurance` — eliminating the lone remaining #119 deferred-creation exception. The change spans the engine (add `assurance.md` to `deferredToSkillAuthoring` in `manager.go`; make `AssuranceContractBlockers` the sole owner of assurance.md existence by skipping it in the generic pre-S3 gates and the repair/doctor bundle-consistency diagnostic; drop the obsolete exclusion rationale comment in `evidence_digests.go`) plus full product-surface alignment (`cmd/instructions.go` assurance guidance, the plan-audit and final-closeout host-skill templates, and end-user docs), with unit tests.

Bundled in (discovered while dogfooding this change, confirmed by the user to fix here): (1) the `slipway repair` / doctor bundle-consistency diagnostic (`internal/state/repair.go`) stays silent on a deferred-absent assurance.md before S3_REVIEW while keeping the S3+/done consistency error; (2) the out-of-scope drift recovery path — `slipway pivot --rescope` is made reachable from S2_EXECUTE/S3_REVIEW/S4_VERIFY (it resets to S0_INTAKE regardless of starting state) so review-time out-of-scope edits can reach the documented recovery, and the `scope_contract_drift` guidance is rewritten to lead with the non-destructive amend-`tasks.md`-`target_files` path and to describe `pivot --rescope` honestly as a reset to S0_INTAKE; `docs/commands.md` is aligned.

Out of scope (explicit exclusions): the #47 scaffold-placeholder floor stays as a backstop (not removed); rescope's S0_INTAKE-reset semantics are unchanged (only its reachable states broaden); no other artifact's timing changes. The assurance enforcement window remains fail-closed at S3_REVIEW and later, and the post-review repair explicitly covers DONE and explicit unknown lifecycle states.

Primary acceptance signal: on standard/strict preset, `assurance.md` does not exist on disk before S3_REVIEW; the host authors it at S3; advancing past S3 stays blocked until it is authored; closeout remains fail-closed for missing/empty/scaffold bodies; the repair diagnostic and rescope reachability behave per the new requirements; `go build/vet/test ./...` green.

Confirmed by user on 2026-06-09 (preset standard for end-to-end dogfooding; scope expanded by user request to bundle the two adjacent governed-surface fixes above).
