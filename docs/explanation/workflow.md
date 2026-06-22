# Workflow

Slipway routes governed work through intake, planning, implementation, review,
and closeout. The active lifecycle state lives in
`artifacts/changes/<slug>/change.yaml`.

For the full workflow document, see [Governed Workflow](../workflow.md). This
page explains the mental model.

## Lifecycle

| Stage | Purpose |
| --- | --- |
| `S0_INTAKE` | Capture intent, scope, open questions, and initial evidence. |
| `S1_PLAN` | Produce research, requirements, decisions, tasks, and plan-audit evidence. |
| `S2_IMPLEMENT` | Execute dependency-ordered task waves and record task evidence. |
| `S3_REVIEW` | Run selected reviewers, repair findings, author assurance, and reach done-ready. |
| `done` | Archive terminal state after done-ready closeout. |

`slipway run` is a shortcut driver. It advances until an operator-facing stop:
a skill handoff, blocker, or done-ready state.

## Read-Only Before Mutation

Use read-only commands to understand the current state:

```bash
slipway status --json
slipway validate --json
slipway next --json --diagnostics
```

Then run the named stage command or complete the surfaced skill. This keeps the
agent from guessing from old context.

## Fail-Closed Recovery

A fail-closed blocker means one of the current-worktree proofs is absent,
stale, malformed, or out of scope. Good recovery does one of these:

- Author the missing artifact from `slipway instructions <artifact>`.
- Rerun the owning stage or selected reviewer in a fresh context.
- Record task evidence through the wave execution path.
- Run bounded local repair only when `health --doctor` names it.
- Start a new governed change if the objective changed.

Bad recovery edits state files by hand, changes timestamps, removes blockers
without fresh proof, or teaches an agent to bypass review.

## Done-Ready Is Not Done

Done-ready means gates have passed and the change can be finalized. `slipway
done --json` archives the terminal state. If `done` reports a dirty-worktree
warning, commit the intended implementation diff together with the archived
Slipway record before removing the worktree.

## Related

- [First governed change](../tutorials/first-governed-change.md)
- [Recover and troubleshoot](../how-to/recover-and-troubleshoot.md)
- [Design](design.md)
