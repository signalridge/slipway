# How To Recover And Troubleshoot

Use this guide when Slipway reports blockers, stale evidence, missing artifacts,
adapter drift, or confusing local state.

The rule is simple: inspect first, then follow the named recovery path. Do not
hand-edit lifecycle authority, evidence verdicts, timestamps, or runtime task
proof.

## Inspect Without Mutating

These commands are the primary Diagnostic JSON surfaces for recovery work.

Run these from the governed worktree:

```bash
git status --short --branch
slipway status --json
slipway validate --json
slipway next --json --diagnostics
```

Use `status` for the lifecycle snapshot, `validate` for gate readiness, and
`next --diagnostics` for the actionable blocker or skill handoff.

## Run Doctor Output

When local state looks inconsistent:

```bash
slipway health --doctor --json
```

Read `applied_repairs`, `unrepaired_drift`, and the named `next_action` fields.
Only run repair when the doctor output matches the issue you see:

```bash
slipway repair --json
```

`repair` is for bounded local integrity issues. It is not a way to force a
change through lifecycle gates.

## Missing Or Stale Task Evidence

Symptoms usually appear in `validate --json` or `next --json --diagnostics` as
missing runtime task evidence, stale execution summaries, or mismatched
freshness inputs.

Safe recovery:

1. Identify the task and required evidence path in the JSON output.
2. Rerun the owning implementation or wave-orchestration handoff.
3. Record task evidence through the owning Slipway command or generated skill.
4. Rerun `slipway validate --json`.

Do not write files under `.git/slipway/runtime/changes/<slug>/evidence/` by
hand.

## Missing Artifact Substance

If a governed artifact is missing, placeholder-only, or structurally invalid,
use the authoring surface named by recovery output:

```bash
slipway instructions requirements --json
slipway instructions decision --json
slipway instructions research --json
slipway instructions tasks --json
slipway instructions assurance --json
```

The command gives the template and quality bar. The authoring skill or human
must write real artifact content from the current objective and source facts.
Copying the template is rejected.

## Review Findings

If review found actionable issues, do not mix review and repair in the same
context. Use the repair surface:

```bash
slipway fix --json
```

Give the returned repair contract to a fresh-context repair agent. After repair,
rerun the affected selected reviewers and then rerun:

```bash
slipway review --json
slipway validate --json
```

Selected reviewer evidence must be fresh for the current suite-result and
execution summary inputs.

## Scope Drift

If `scope_contract` reports changed files outside a task's `target_files`,
choose one safe path:

- Revert or move the out-of-scope change yourself if it was accidental.
- Amend the same-intent task or artifact through the surfaced Slipway planning
  or review path.
- Start a new governed change if the objective changed.

Do not hide a changed file by editing evidence.

## Dirty Worktrees After Done

`slipway done --json` can archive a done-ready change while returning a
`worktree_dirty_warning` for non-active files that still need committing.

Safe recovery:

```bash
git status --short
git diff --check
```

Commit the intended implementation diff together with the archived Slipway
record. The active bundle is rewritten into `artifacts/changes/archived/<slug>/`.

## Adapter Drift

If generated AI-tool commands or skills look stale:

```bash
slipway init --refresh
```

Then inspect the diff:

```bash
git status --short .claude .codex .cursor .gemini .opencode
```

Generated adapters are handoff aids. If adapter behavior and CLI behavior
disagree, trust the current-worktree CLI output and refresh the generated files.

## Recovery Quick Reference

| Symptom | Inspect | Safe action |
| --- | --- | --- |
| Unsure what to do next | `slipway next --json --diagnostics` | Follow returned skill, blocker, or command. |
| Gate says stale evidence | `slipway validate --json` | Rerun the owning stage, reviewer, or task evidence path. |
| Local state looks corrupt | `slipway health --doctor --json` | Run `slipway repair --json` only for named bounded repairs. |
| Artifact is placeholder-only | `slipway instructions <artifact> --json` | Author real content and rerun validation. |
| Review found issues | `slipway fix --json` | Repair in a fresh context, rerun affected reviewers. |
| Adapter files are stale | `slipway init --refresh` | Inspect generated diff and preserve user-owned files. |

## Related

- [Commands](../reference/commands.md)
- [Workflow](../explanation/workflow.md)
- [Operator Guide](../operator-guide.md)
