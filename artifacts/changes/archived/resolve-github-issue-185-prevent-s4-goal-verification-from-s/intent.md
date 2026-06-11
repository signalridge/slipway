# Intent

## Summary
Resolve GitHub issue #185: prevent S4 goal-verification from self-staling when evidence_refs updates change.yaml
## Complexity Assessment
complex
Rationale: the bug sits in governed evidence freshness for S4 verification and
can block `G_ship`; a fix must preserve fail-closed stale evidence behavior
while excluding only engine-owned evidence pointer churn.

## Guardrail Domains
None detected. This does not touch auth, credentials, PII, financial flows,
schema migrations, irreversible operations, or external API contracts.

## In Scope
- Fix GitHub issue #185: an S4 required skill must not become stale solely
  because `slipway evidence skill` records its own `evidence_refs` pointer in
  `artifacts/changes/<slug>/change.yaml`.
- Cover the `goal-verification` and `final-closeout` S4 evidence path enough to
  prove the public recovery flow can reach `done_ready` without editing
  engine-owned evidence by hand.
- Add regression coverage in the existing Go test surface for evidence digest
  freshness and recovery behavior.
- Keep normal stale detection for real `change.yaml` content changes.

## Out of Scope
- Adding a new manual `slipway evidence restamp` command.
- Editing Lattice governed artifacts or runtime evidence manually as a
  workaround.
- Weakening stale evidence checks for ordinary target files or unrelated skill
  inputs.
- Retiring existing recovery behavior outside the self-stale `change.yaml`
  evidence-reference case.

## Constraints
- Use the current Slipway CLI behavior as authority and keep verification
  records engine-owned.
- The fix must be narrow enough that stale evidence still fails closed when
  user-authored artifacts or implementation files change.
- Codebase map files in this worktree appear stale from previous issues, so
  research must verify the actual #185 code paths directly.

## Acceptance Signals
- A focused regression proves recording `goal-verification` evidence does not
  immediately produce `required_skill_stale:goal-verification:.../change.yaml`
  when the only `change.yaml` delta is the evidence pointer.
- A companion assertion proves a meaningful `change.yaml` change still makes
  the required skill stale.
- Targeted package tests for the touched evidence/freshness code pass.
- `go test -count=1 ./...`, `git diff --check`, and `go run . validate --json`
  pass from the #185 worktree.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] `certifiedSkillInputDigest` in
  `internal/engine/progression/evidence_digests.go` owns the S4 digest inputs.
  The fix normalizes current-change `change.yaml` inputs by hashing a strict
  decoded `model.Change` with `EvidenceRefs` cleared, preserving stale detection
  for meaningful authority changes without changing `evidence skill` write
  order.

## Deferred Ideas
- A public restamp/recovery command for broader Tier-0 stale evidence cases can
  be considered separately if the narrow self-stale fix is insufficient.

## Approved Summary
Approved from the user-provided objective to fix GitHub issue #185 and the live
issue body. The change will repair the S4 `goal-verification`/`final-closeout`
self-stale loop caused by `change.yaml` evidence pointer updates, preserve
fail-closed stale detection for real input changes, and prove the behavior with
focused Go regression tests plus normal repo verification. Out of scope: manual
Lattice artifact edits, adding a new restamp command, and weakening unrelated
stale evidence gates.
