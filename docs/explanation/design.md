# Design

Slipway is a small governance control plane for local AI-assisted development.
It does not replace an AI coding tool, project tracker, or Git. It makes agent
work legible by binding every governed change to a lifecycle, a current
authority file, and evidence that can be inspected after the session ends.

For the full design document, see [Design Philosophy](../design.md). This page
summarizes the concepts that matter most when deciding whether to use Slipway.

## Core Principles

| Principle | Meaning |
| --- | --- |
| Local-first | The repo contains active state and audit records. |
| One authority | `change.yaml` owns current lifecycle state; logs explain how it changed. |
| Bounded autonomy | Agents can advance work, but gates, blockers, review, and done-ready proof stay visible. |
| Adapter thinness | Host files route to the CLI instead of becoming independent workflow engines. |
| Artifact traceability | Intent, research, requirements, decisions, tasks, evidence, review, and assurance stay connected. |
| Fresh verification | Completion is valid only when current evidence proves the current worktree state. |

## Why Evidence Is Re-Derived

Slipway does not trust a verdict just because a file says `pass`. It derives
freshness from authoritative inputs such as the current bundle, task plan,
execution run version, selected review set, the terminal `ship-verification`
suite run, and runtime task evidence.

That is why stale proof fails closed. Recovery is to rerun the owning stage,
reviewer, or task evidence path, not to restamp files.

## Why Worktrees Matter

Governed changes often bind to dedicated worktrees. The current worktree is the
behavioral surface. A root checkout, old branch comparison, or archived bundle
can help with review, but it is not a substitute for fresh `status`, `validate`,
and `next` output from the owning worktree.

## Why Adapters Are Thin

Generated Claude, Codex, Copilot, Cursor, Kilo, Kiro, OpenCode, Pi, Qwen, and
Windsurf files help an AI tool find the right command or skill. They do not own
lifecycle semantics. If a generated adapter and the CLI disagree, refresh the
adapter and trust the current CLI.

## Related

- [Workflow](workflow.md)
- [Commands](../reference/commands.md)
- [AI tool adapters](../reference/ai-tools.md)
