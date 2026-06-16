# Intent

## Summary
Fix issue #238: resolving an Open Question in `intent.md` during research must
not stale S0 intake-clarification evidence or force Approved Summary
reconfirmation when substantive intake scope text is unchanged.

## Complexity Assessment
complex
Rationale: this touches Slipway freshness/digest authority boundaries and must
preserve fail-closed behavior for real scope changes while avoiding a
disproportionate S0 recovery for research-owned Open Question resolution.

## Guardrail Domains
Governance lifecycle correctness

## In Scope
- Investigate the current intake-clarification freshness input/digest path for
  `intent.md`.
- Change the intake freshness behavior so Open Questions resolution state
  changes, such as `- [ ]` to `- [x]` and resolution notes under Open Questions,
  do not invalidate S0 intake-clarification evidence when substantive intake
  sections and Approved Summary text are unchanged.
- Keep Open Questions routing semantics intact: unchecked checklist items still
  route to research, and resolved items no longer block research completion.
- Add regression coverage for issue #238 and adjacent negative coverage showing
  substantive intake changes still stale the relevant upstream evidence.
- Update any user-facing lifecycle/recovery surface only if the code change
  requires it for clarity.

## Out of Scope
- Broad redesign of Slipway evidence freshness, lifecycle stages, or recovery
  classes.
- Any bypass/force-close path for stale evidence recovery.
- Changing the meaning of substantive intake sections such as Summary, In Scope,
  Out of Scope, Constraints, Acceptance Signals, or Approved Summary.

## Constraints
- Use the current worktree's Slipway CLI behavior as the authority.
- Do not hand-edit engine-owned freshness state or verification verdicts.
- Preserve strict fail-closed governance for real intake scope or Approved
  Summary changes.
- Keep the implementation scoped to issue #238.

## Acceptance Signals
- A focused regression test demonstrates that checking off or documenting a
  resolved Open Question in `intent.md` does not produce
  `stale_evidence_recovery_available: S0_INTAKE` for unchanged substantive
  intake fields.
- A negative regression test demonstrates that changing a substantive intake
  section still stales intake-clarification evidence.
- Relevant Go tests pass in the dedicated worktree.
- Governed change reaches done-ready and is finalized with fresh evidence.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Where is the narrowest authority boundary for intake-clarification digest
  inputs: artifact digest canonicalization, skill input mapping, or freshness
  comparison?
  Resolved: the narrowest boundary is the skill input mapping in
  `certifiedSkillInputDigest`; intake-clarification should use an
  intake-specific `intent.md` digest while research-orchestration and
  plan-audit keep the full governed file input.
- [x] Which existing test fixture best reproduces issue #238 without relying on
  brittle wall-clock or manually stamped freshness state?
  Resolved: `StaleEvidenceRecoveryAvailable` with a stamped
  intake-clarification digest directly reproduces the S0 reopen trigger, and a
  paired substantive Summary edit covers the negative path.

## Deferred Ideas
- Let research-orchestration own Open Questions resolution writes explicitly if
  a future lifecycle surface needs stronger ownership metadata.

## Approved Summary
Confirmed by user at 2026-06-16T07:05:14Z.

Fix issue #238 by changing Slipway intake freshness so research-stage Open
Questions resolution in `intent.md`, such as checking `- [ ]` to `- [x]` or
adding a resolution note, does not stale S0 intake-clarification evidence when
the substantive intake sections and Approved Summary text are unchanged.
Preserve fail-closed stale-evidence behavior for real changes to Summary,
scope, constraints, acceptance signals, or Approved Summary. Acceptance requires
focused regression coverage for both the allowed Open Questions resolution path
and the negative substantive-change path, plus passing relevant Go tests and
governed finalization.
