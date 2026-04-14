---
skill_id: differential-review
domain: review-change-shape
function: review the delta against the baseline rather than the whole file
tier: T1
primary_attachment: procedure
summary: "Use when reviewing a diff against a known baseline. Triggers on review command or PR-shaped change context."
trigger_signals:
  - command: review
    reason: "review command invoked; scope reviewer attention to the diff"
  - user_text_matches: ["diff", "pull request", "delta"]
    reason: "User text names a diff-shaped review"
evidence_contract: verdict
bindings:
  - type: command-manual
    target: review
    attachment: procedure
  - type: command-manual
    target: review
    attachment: checklist
provenance_ref: provenance.yaml
---

# Differential Review

```
IRON LAW: REVIEW THE DELTA, NOT THE FILE
```

## Purpose
A reviewer's attention is finite; spend it on the change, not the surrounding
code. Differential review bounds scope to the diff plus the minimum context
needed to judge it, and labels every finding as "caused by this change" or
"pre-existing".

## Procedure
1. List the hunks. Read each hunk with just enough surrounding context to
   judge intent.
2. For each hunk, ask: does this change preserve the invariants the baseline
   relied on? Cite the invariant.
3. Label every finding: `new` (introduced by this change), `pre-existing`
   (latent before this change), or `worsened` (exposed by this change).
4. Only `new` and `worsened` findings are blockers. `pre-existing` findings
   become tickets, not blockers.
5. Before signing off, re-read the smallest hunks last; small hunks hide
   the highest-risk changes.

## Checklist
- [ ] Each hunk judged with minimum surrounding context.
- [ ] Each finding labelled `new`, `pre-existing`, or `worsened`.
- [ ] Blockers are only `new` or `worsened`.
- [ ] Pre-existing findings captured as tickets, not merge blockers.
- [ ] Small hunks re-read last.

## Anti-patterns
- Blocking a PR on pre-existing technical debt discovered during review.
- Scanning entire files and forming opinions outside the diff.
- Treating small hunks as trivially safe.
