# Concerns

Re-authored for change `resolve-issue-163-decisions-gate` (GitHub issue #163).

- Load-bearing invariant: a decision that is explicitly superseded or deprecated
  must not be treated as a usable planning authority, even if its required
  sections contain authored prose.
- Status-vs-locking risk: `G_plan` currently controls pending-vs-locked
  decision visibility. The new status gate must not conflate "not yet locked"
  with "dead decision"; these are separate axes.
- Silent continuation risk: `parseDecisionItems` currently returns only strings
  and has no way to report "decision exists but is rejected". If this remains
  string-only, `next` can silently omit or surface decision text instead of
  emitting an actionable blocker.
- Unknown status risk: issue #163 asks for a defined status taxonomy. Treating an
  unrecognized explicit status as accepted would be unsafe because a misspelled
  dead status could bypass the gate.
- Compatibility risk: older `decision.md` files may omit any status section. A
  missing status should remain compatible while still allowing authored selected
  approaches; explicit unknown statuses should be blocked once status parsing is
  introduced.
- Recovery risk: a new blocker requires canonical reason-code and recovery prose
  so operators get a clear path: revise or replace the dead decision before
  continuing.
- Scope risk: GSD's issue-number ADR filenames and append-only amendments are
  useful but optional in issue #163. Combining them with the status gate would
  broaden the change and make the acceptance signal harder to verify.
