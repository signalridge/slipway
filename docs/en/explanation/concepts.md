# Core concepts

Slipway separates user intent, host execution, and durable Run state. These terms are enough to understand the product without reading the machine protocol.

![Slipway Run lifecycle: explicit start enters a one-Action-at-a-time Action and Outcome loop; user controls can skip, pause, stop, or resume; ended only means the automatic Action queue is empty.](../../assets/diagrams/lifecycle.svg)

## Run

A **Run** is one interruptible attempt to carry out a goal in one Git worktree. It has a bounded Action budget, recovery history, and an immutable starting workspace identity. A task may have several Runs; a Run has at most one primary source.

Starting a Run is always explicit. Slipway does not watch chat, infer that ordinary conversation should become work, or start through an ambient hook.

## Action and Outcome

The CLI returns one **Action** at a time:

| Action | Host responsibility |
| --- | --- |
| `orient` | Inspect repository facts, Git state, and conventions; choose the next useful step. |
| `clarify` | Ask for one genuine human decision only when repository facts cannot settle it. |
| `implement` | Perform the bounded technical change and report actual activities and files. |
| `review` | Read the observed change and report intent or quality findings without editing. |
| `summarize` | Consolidate observed changes, activities, findings, known issues, and uncertainty. |

Review is enabled by default but is issued only after Slipway observes code changes. `--no-review` disables it for the Run.

The host answers with a structured **Outcome**. Slipway validates the Outcome, records it, observes Git independently, and chooses the next Action. Reported activity does not prove success, and an observed diff does not prove which process created it.

## Source

A Run can have either source:

- **Ad hoc:** the user-provided goal is the source.
- **GitHub Change:** the host imports a self-contained Change Issue whose accepted sections are pinned by digest.

A source snapshot does not silently change during a Run. If a Change Issue is refreshed and its accepted content differs, Slipway pauses for an explicit keep-or-adopt choice. A history fork requires a new Run.

## Objective and Change

An **Objective** is optional planning structure for an outcome that needs several independently deliverable Changes. It is never executable.

A **Change** is a self-contained work item with one coherent result. It must carry everything needed for its own implementation; a Run does not resolve requirements by reading a parent Objective at execution time.

These terms do not depend on whether the GitHub repository is owned by a personal account or an organization. See [Using GitHub Issues](../guides/github-issues.md).

## Budget and pauses

The Action budget limits how many Actions the CLI can issue before pausing; it is not a time estimate or a quality score. A Run also pauses for a human decision, source choice, unavailable environment, or exact destructive confirmation.

Users may skip an Action, stop a Run, resume it, reorder work, or take over. A skip needs no reason. Any resume revalidates the original worktree identity and invalidates stale pending work as required by the protocol.

## Review and completion

Review is advisory and read-only. Findings flow to the summary; Slipway does not automatically repair and review again.

`ended` means the automatic Action queue is empty. It does not mean that tests passed, review found nothing, branch protection is satisfied, a pull request is approved, or a release is ready. Slipway reports those facts so the user and repository policy can decide what happens next.

## Local state

Recovery data is stored below `<git-common-dir>/slipway/runs/`. The append-only journal records transitions; a replaceable projection accelerates reads; accepted issue sections are stored as content-addressed materials. See [Runs, recovery, and privacy](../guides/runs-and-recovery.md).

For exact JSON shapes, read the [machine protocol](../reference/machine-protocol.md).
