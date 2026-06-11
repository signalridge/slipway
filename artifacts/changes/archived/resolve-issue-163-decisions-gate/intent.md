# Intent

## Summary
Resolve GitHub issue #163 by making Slipway decision artifacts machine-parsed
for lifecycle status and failing closed when a downstream stage would build on a
dead decision.

## Complexity Assessment
complex

Rationale: the change touches governed artifact parsing, readiness or handoff
gates, reason-code/recovery surfaces, and tests that must prove fail-closed
behavior for a lifecycle contract.

## Guardrail Domains
None detected. The change affects governance correctness but does not modify
auth, credentials, PII, financial flows, schema migration, irreversible
operations, or external API contracts.

## In Scope
- GitHub issue #163: `feat(decisions): machine-parsed decision artifacts +
  superseded status gate`.
- Parse Slipway `decision.md` artifacts into a structured contract that exposes
  selected approach text and lifecycle status.
- Define the Slipway decision status taxonomy needed for this gate, including
  accepted/proposed-style live statuses and superseded/deprecated dead statuses.
- Add a `shouldRejectStatus`-style helper so superseded and deprecated
  decisions are rejected before a governed stage can rely on them.
- Integrate the dead-decision gate with the existing `decision.md` contract and
  lifecycle readiness/handoff path so building on a superseded decision fails
  closed with actionable diagnostics.
- Add focused unit tests and property/fuzz-style coverage for parser/status
  normalization and the fail-closed superseded/deprecated gate.
- Use local GSD Core only as a reference for ADR parsing/status rejection:
  `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/adr-parser.cts` and
  `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/adr/README.md`.

## Out of Scope
- Do not adopt GSD's implementation wholesale or rename Slipway decisions to
  ADRs.
- Do not require issue-number-prefixed decision filenames in this change; issue
  #163 lists that as optional and it is independent of the fail-closed gate.
- Do not implement append-only dated amendments for Slipway decision files in
  this change; keep it as a possible follow-up after the status gate is proven.
- Do not change unrelated governed artifact schemas or lifecycle stages except
  where needed to parse and reject dead decisions.

## Constraints
- Current Slipway CLI behavior and tests are authoritative; GSD is a reference,
  not an upstream compatibility target.
- Preserve the existing #119 empty-floor decision contract: template-only or
  missing decisions must still be handled by the existing contract paths.
- The gate must fail closed for dead statuses and avoid silently treating an
  unknown or malformed status as an accepted locked decision.
- Reason codes and recovery guidance must remain actionable for operators.

## Acceptance Signals
- `decision.md` with a superseded or deprecated status produces a blocking
  governed diagnostic before a downstream stage can build on its selected
  approach.
- Parser unit tests cover heading/status aliases, selected approach extraction,
  live statuses, dead statuses, and malformed/unknown status handling.
- Property or fuzz-style tests exercise status normalization and prove rejected
  status variants remain rejected across casing, spacing, punctuation, and
  heading forms.
- Existing decision contract tests still pass, including template-only,
  missing-file, unreadable-file, and pending-vs-locked decision behavior.
- `go test -count=1 ./...`, `go run . validate --json`, and the Slipway
  lifecycle gates selected for this change pass with fresh evidence.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Confirm the exact integration point for refusing a dead decision:
  decision contract validation, `next` skill constraints, readiness, or a shared
  parsed-decision helper used by those surfaces. Resolution: use a shared parsed
  decision contract under `internal/engine/artifact`, then consume it from both
  `DecisionContractBlockers` and `cmd/next_skill.go`.
- [x] Confirm whether unknown decision statuses should block only when a
  decision is otherwise being surfaced as pending/locked, or block as soon as a
  status field exists but is unrecognized. Resolution: missing status remains
  compatible, while explicit unknown status blocks because the issue requires a
  defined status taxonomy and fail-closed behavior.

## Deferred Ideas
- Issue-number-prefixed decision filenames, mirroring GSD's ADR naming strategy.
- Append-only dated amendments for historical decision updates.
- Richer decision ingestion beyond the fields needed for the superseded status
  gate.

## Approved Summary
User objective on 2026-06-11 authorizes using the governed workflow to solve all
of GitHub issue #163 through done-ready and to make best-effort choices if
blocked. This change will parse Slipway `decision.md` status and selected
approach, define the dead-status rejection taxonomy, and fail closed when a
superseded or deprecated decision would otherwise be used by governed
progression. Optional GSD-style issue-number filenames and append-only
amendments are deferred because they are not required by the issue's acceptance
signal: "Building on a superseded decision fails closed; parser unit+property
tested."
