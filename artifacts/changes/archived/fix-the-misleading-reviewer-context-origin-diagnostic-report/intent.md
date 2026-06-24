# Intent

## Summary
Fix the misleading reviewer context-origin diagnostic reported in issue #319. When reviewer evidence records multiple context_origin:stage=fix handles (one per fresh-context repair subagent across S3 fix batches, exactly as the slipway fix command instructs) alongside the single context_origin:stage=review handle, `slipway validate` falsely reports that the selected reviewer recorded no context-origin handle and tells the user to re-run the reviewer in a fresh native subagent. Root cause: model.ContextOriginHandlesFromVerification (internal/model/context_attestation.go) treats every stage as single-valued and fails the entire record closed when any single stage carries multiple distinct handles; because the fix stage is inherently multi-valued, the multiple fix handles poison the parse so the unique, well-formed review handle becomes unreadable. The same single-valued constraint also defeats the fix-handle HandleSet collection in crossStageContextParticipants (internal/engine/progression/authority.go), which already expects a multi-valued fix participant. Make the documented review+fix evidence shape validate cleanly: parse fix as a multi-valued stage with set semantics while keeping single-valued stages (review, plan_origin, audit_origin) fail-closed on genuinely conflicting handles, so multiple fix handles no longer mask the review handle and the cross-stage independence lattice receives the full fix handle set. Keep the slipway fix command instructions aligned with the accepted shape. Must not weaken fail-closed safety for single-valued stages or introduce any attestation bypass.

## Complexity Assessment
complex
<!-- Rationale: -->
Small code surface but safety-sensitive: the change edits the fail-closed grammar
that feeds the cross-stage context-independence lattice (a governance safety gate).
It must distinguish single-valued stages (still fail-closed on conflict) from the
inherently multi-valued fix stage, touch multiple consumers in authority.go, keep
generated fix instructions aligned, and add regression coverage — all without
introducing any attestation bypass.

## Guardrail Domains
<!-- none detected -->
None classified. The work is internal governance evidence-validation logic, not
Auth/Credentials/Financial/Schema/Irreversible/External-API. It nonetheless
touches fail-closed Review-And-Safety logic, so it must remain fail-closed for
single-valued stages.

## In Scope
- `internal/model/context_attestation.go`: make context-origin handle parsing
  distinguish single-valued stages from the multi-valued `fix` stage. `fix` may
  carry multiple distinct handles (set semantics); `review`, `plan_origin`,
  `audit_origin` stay single-valued and fail closed on genuinely conflicting
  handles. Ensure `ReviewContextOriginHandleFromVerification` resolves the unique
  review handle even when multiple `stage=fix` handles are present in the same record.
- `internal/engine/progression/authority.go`: `crossStageContextParticipants`
  correctly collects every `stage=fix` handle on each reviewer record into the
  fix `HandleSet` (no longer dropped by the single-valued constraint), and stops
  emitting the false `context_origin_handle_invalid` "no context-origin handle
  for selected reviewer" blocker when only the multi-fix shape is present.
- `internal/tmpl/templates/_partials/command-fix-body.tmpl`: align the fix
  instruction text so it is explicit that a reviewer's evidence may accumulate
  multiple `context_origin:stage=fix` handles (one per repair subagent / batch).
- Regression tests: `internal/model/context_attestation_test.go` (multi-fix +
  single-review coexistence; fix set extraction; single-valued stages still fail
  closed) and the authority-layer participant/blocker tests (multi-fix reviewer
  evidence no longer false-flags the review handle; fix HandleSet is complete).
- Regenerate any generated skill/command surface affected by the tmpl edit.

## Out of Scope
- No change to the single-valued fail-closed semantics of `review`,
  `plan_origin`, `audit_origin`, or the executor handle-set stage.
- No new attestation bypass, force-close, restamp, or private-attestation path.
- No refactor of unrelated lattice logic or the wave-execution token grammar.
- Not fixing issue #263 (recovery dead-end replacing already-passing invalid
  context-origin evidence) — related but distinct.
- No change to ship-verification (#322) gate behavior beyond what the parsing fix
  transitively corrects.

## Constraints
- Must preserve fail-closed safety: single-valued stages still reject conflicting
  handles; only genuinely multi-valued stages (fix) accept a handle set.
- Smallest clean design; keep code, generated skills, docs, and instructions
  aligned as one product surface.
- Full `go test ./...` green; gofmt -s and golangci-lint (incl. gofmt simplify) clean.

## Acceptance Signals
- New unit test: a reviewer record carrying multiple distinct
  `context_origin:stage=fix=<h>` references plus one `context_origin:stage=review=<h>`
  resolves the review handle (ok=true) instead of failing closed.
- The fix-stage handles are extractable as a complete set; `review`/`plan_origin`/
  `audit_origin` still fail closed on two different handles for the same stage.
- `crossStageContextParticipants` no longer emits the
  `context_origin_handle_invalid` reviewer-missing blocker for the multi-fix shape,
  and the fix participant `HandleSet` contains every recorded fix handle.
- End-to-end: reviewer evidence with multiple fix handles + one review handle no
  longer produces "recorded no context-origin handle for selected reviewer" from
  `slipway validate`.
- fix command instruction text matches the accepted evidence shape.
- Full test suite and lint pass from the current worktree.

## Open Questions
None.

## Deferred Ideas
- Consider a small explicit single-valued-vs-multi-valued stage classification
  table if more multi-valued stages emerge later; not needed for this change.

## Approved Summary
Confirmed 2026-06-24T12:32:14Z by user.

This change fixes the misleading reviewer context-origin diagnostic (#319) by making
the context-origin handle grammar treat `fix` as a multi-valued stage — a reviewer's
evidence may carry one `context_origin:stage=fix` handle per repair subagent — while
keeping `review`, `plan_origin`, and `audit_origin` single-valued and fail-closed on
genuinely conflicting handles. Multiple fix handles therefore no longer mask the
unique reviewer handle, `slipway validate` stops falsely reporting "recorded no
context-origin handle for selected reviewer", and the cross-stage independence lattice
receives the complete fix handle set.

Scope: `internal/model/context_attestation.go` parsing, the
`internal/engine/progression/authority.go` fix-handle collection and reviewer-missing
blocker, the `slipway fix` command instruction text, and regression tests.

Out of scope: single-valued stages keep their fail-closed semantics; no attestation
bypass/force-close/restamp; issue #263 is not addressed.

Primary acceptance signal: reviewer evidence with multiple `stage=fix` handles plus
one `stage=review` handle validates cleanly (the review handle resolves, no
reviewer-missing blocker), with the full Go test suite and lint green.
