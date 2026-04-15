---
skill_id: git-recovery
domain: repair-ci
function: recover git state without destroying unsaved work or bypassing hooks
tier: T2
primary_attachment: procedure
summary: "Use when git state is entangled and a destructive operation is being considered. Triggers on repair or status commands or user text naming git recovery."
trigger_signals:
  - command: ["repair", "status"]
    reason: "repair or status command invoked; git recovery may be in scope"
  - blocker_reason: ["worktree_dirty", "branch_diverged", "detached_head"]
    reason: "Blocker cites an entangled git state"
  - user_text_matches: ["git reset", "git rebase", "--no-verify", "force push", "detached head"]
    reason: "User text names a destructive or high-risk git operation"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: repair
    attachment: procedure
  - type: command-auto
    target: status
    attachment: checklist
  - type: host-embedded
    target: worktree-preflight
    attachment: procedure
---

# Git Recovery

```
IRON LAW: NEVER DESTROY WHAT HASN'T BEEN MIRRORED
```

## Purpose
Git state entanglement (dirty worktree, diverged branch, detached HEAD,
failed rebase) tempts destructive shortcuts. Those shortcuts erase work.
Recover without destroying, and never bypass hooks to make state go away.

## Procedure
1. **Snapshot first.** Create a recovery branch or backup stash that points at
   the current HEAD + worktree + index. Every subsequent step is safer
   because of this.
2. **Classify the entanglement.**
   - Dirty worktree: stash or commit; never `checkout .` or `reset --hard`
     without a snapshot.
   - Diverged branch: rebase on top of the target (not force-reset).
   - Detached HEAD: create a named branch before doing anything else.
   - Failed rebase: `git rebase --abort` is the default; resume only if the
     in-flight state is understood.
3. **Never `--no-verify`.** If a hook fails, investigate and fix the
   underlying issue. Bypass only with explicit user authorization, recorded.
4. **Never force-push main/master.** For shared branches, coordinate in
   writing before force-push; for personal PR branches, force-push is
   acceptable after a rebase, still with snapshot.
5. **Verify.** After recovery, diff against the snapshot branch; list what
   changed and confirm no file was dropped unexpectedly.

## Checklist
- [ ] Snapshot branch or stash created before any destructive op.
- [ ] Entanglement classified.
- [ ] No `--no-verify` used without explicit authorization.
- [ ] No force-push to shared branches without coordination.
- [ ] Post-recovery diff against snapshot verified.

## Anti-patterns
- `git reset --hard` before snapshotting.
- `--no-verify` to silence a failing hook.
- Force-pushing main/master "to get unstuck".
