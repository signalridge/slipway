---
skill_id: review-comment-triage
domain: repair-ci
function: triage reviewer comments into accept, push-back, or defer with a written disposition
tier: T2
primary_attachment: procedure
summary: "Use when addressing reviewer comments on an open PR. Triggers on fix command or user text naming PR review comments."
trigger_signals:
  - command: fix
    reason: "fix command invoked; reviewer comments may be in scope"
  - user_text_matches: ["review comment", "pr comment", "address comment"]
    reason: "User text names PR review comments"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: fix
    attachment: procedure
---

# Review Comment Triage

```
IRON LAW: EVERY COMMENT GETS A WRITTEN DISPOSITION
```

## Purpose
Reviewer comments are not a to-do list; they are claims. Each claim needs a
disposition the reviewer can read: accept, push-back, or defer. Silent
cosmetic-only fixes produce surprise blockers on the next round.

## Procedure
1. Enumerate every open comment thread. Do not batch.
2. For each comment, classify the disposition:
   - `accept` — the change will apply the requested fix.
   - `push-back` — the requested fix is wrong or out of scope; cite why and
     propose an alternative (or decline cleanly).
   - `defer` — the point is valid but better suited to a follow-up; file the
     ticket and link it in the reply.
3. For `accept`, apply the fix and cite the resulting commit/hunk in the
   reply. Do not close the thread without a visible change.
4. For `push-back`, reply with the reason before making the change; invite
   the reviewer to disagree.
5. For `defer`, the ticket link goes in the reply, not in the PR body alone.
6. Resolve threads only after the disposition is written.

## Checklist
- [ ] Every open thread has a disposition recorded.
- [ ] Accepted comments cite the commit/hunk.
- [ ] Push-backs cite the reason before the change.
- [ ] Defers link to a ticket.
- [ ] No silent cosmetic fixes without reply.

## Anti-patterns
- Pushing commits that silently address some comments and ignore others.
- Closing threads without a reply.
- Replying "done" without a commit/hunk link.

## Helpers
GitHub helpers default to `--backend auto`: use authenticated `gh` when
available, and use the token-backed API when `gh` is unavailable or reports an
auth-required error while `GH_TOKEN` or `GITHUB_TOKEN` is set. Use `--backend gh`
to require GitHub CLI, or `--backend api` to require token API. Each helper
fails closed when no authenticated backend is available. No generated Python,
shell, or `jq` helper script is required.

- `slipway tool fetch-pr-feedback --repo owner/repo --pr N` — fetch and
  LOGAF-categorize review comments / threads for a PR. Read-only.
- `slipway tool fetch-review-requests --org ORG --teams team-a,team-b` —
  list open review requests for a user across team memberships. Read-only.
- `slipway tool reply-to-thread THREAD_ID BODY` — reply to a PR review
  thread. **Write side effect when confirmed.** Defaults to dry-run: without
  `--confirm` it prints the intended GraphQL mutation and intentionally exits
  non-zero with `reply_to_thread_dry_run`, so automation cannot mistake a preview
  for a posted reply. Only supply `--confirm` once the reply body and thread id
  have been reviewed. Reply bodies get a `Slipway` attribution by default; pass
  `--signature <label>` to customize it or `--signature ""` to disable it.
