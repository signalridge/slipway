# Intent

## Summary
Fix the confirmed #310/#311 evidence-recording and recovery UX defects that
cause Slipway to accept unusable task evidence, present ready states as blocked,
or recommend ambiguous deletion for archived-change residue.
## Complexity Assessment
simple
Rationale: the change is limited to CLI/governance recovery surfaces and focused
regression coverage. It does not introduce a new workflow stage, external API, or
guardrail-domain behavior.

## In Scope
- Fail fast during `slipway evidence task --result-file` import when duplicate
  non-empty `session_id` values in the active run would later invalidate
  `wave-orchestration` evidence.
- Improve malformed engine-owned `verification/<skill>.yaml` diagnostics so
  users are directed to move free-form notes to
  `verification/<skill>-notes.md` and rerun `slipway evidence skill --notes-file`.
- Adjust `slipway next --json --diagnostics` ready/no-skill-required output so
  it does not use `blocked_by_governance` or "resolve governance blockers"
  wording when the lifecycle can advance.
- Improve orphaned active bundle residue recovery for slugs that also have an
  archived change record, so the delete target is explicitly active residue and
  the archive/source commits are not implied as deletion targets.
- Exclude lifecycle bookkeeping (run-summary version, per-wave parallel flag,
  generated-at timestamp) from the `wave-orchestration` wave-plan freshness
  digest so a freshly recorded `slipway evidence skill --skill wave-orchestration`
  pass does not immediately stale itself when the wave plan re-materializes at a
  different run version (#310.1, reproduced on current `main`).
- State the engine-owned `verification/<skill>.yaml` boundary preventively in the
  generated `wave-orchestration` skill guidance, not only in the malformed-record
  error path.
- Add regression tests for each confirmed behavior.

## Out of Scope
- Do not broaden the `wave-orchestration` wave-plan digest change beyond excluding
  the named bookkeeping fields; structural, scope, and semantic task-plan hashes
  MUST still stale the evidence when the plan genuinely changes.
- Do not change `slipway delete` destructive semantics beyond safer recovery
  classification and wording.
- Do not modify archived change records or user worktrees as part of the fix.

## Constraints
- Keep compatibility with existing reason-code taxonomy where possible.
- Preserve fail-closed behavior for malformed engine-owned verification records.
- Keep recovery commands explicit and non-destructive by default when archive or
  worktree safety context is ambiguous.

## Acceptance Signals
- Focused regression tests show duplicate result-file `session_id` imports are
  rejected before task evidence is persisted.
- Focused regression tests show malformed `verification/<skill>.yaml` errors
  mention the engine-owned boundary and `--notes-file` recovery path.
- Focused regression tests show ready/no-skill-required `next --json
  --diagnostics` output avoids governance-blocked wording.
- Focused regression tests show archived same-slug orphaned active residue
  recovery names active residue explicitly and does not recommend `--worktree`.
- A focused regression test shows a freshly stamped `wave-orchestration` pass
  stays fresh after the wave plan re-materializes at a new run version, while a
  real task-plan scope change still stales the evidence.
- `go test ./cmd ./internal/model ./internal/state ./internal/engine/progression`
  passes, or any narrower test scope is justified by the final evidence.

## Open Questions
None.

## Approved Summary
User requested a complete governed repair after reproducing the open issues, then
directed that the #310.1 wave-plan digest-stale path also be reproduced and fixed.
That path was reproduced on current `main` (a freshly stamped
`wave-orchestration` evidence immediately re-stales itself when the wave plan
re-materializes at a different run version), so the approved scope now covers the
confirmed #310/#311 defects plus the #310.1 wave-plan freshness-digest fix and the
preventive generated-skill boundary guidance, each with focused regression
coverage. The user chose to land this through a governed re-walk rather than a
direct patch.
