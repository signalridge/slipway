# Slipway Documentation

Slipway is a governance CLI for AI-assisted software delivery. It gives local AI-agent work a durable lifecycle, a current-state authority, and evidence-backed closeout inside the repository where the work happens.

Use this page as the map for the documentation set.

| Page | Audience | Use it for |
| --- | --- | --- |
| [Installation](installation.md) | New users, AI coding tools | Choose the right platform package, install or build Slipway, initialize a repo, and generate adapters. |
| [Design Philosophy](design.md) | Evaluators, maintainers, advanced users | Understand why Slipway keeps CLI authority, artifact evidence, and AI-tool adapters separate. |
| [Governed Workflow](workflow.md) | Users and operators | Run intake, planning, execution, review, and done-ready closeout. |
| [Command Reference](commands.md) | All users | Inspect command classes, JSON surfaces, and common flags. |
| [AI Tool Adapters](ai-tools.md) | AI-tool users and integrators | Understand generated skills, commands, hooks, and invocation spellings. |
| [Operator Guide](operator-guide.md) | Maintainers and agents | Read state authority, worktree, verification, health, and recovery guidance. |
| [Contributing](contributing.md) | Contributors | Work safely in the Go codebase and docs system. |

## Mental Model

Slipway keeps these core surfaces separate:

| Surface | Role |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | Current lifecycle and routing authority; archived snapshots are Git-safe project records. |
| `artifacts/changes/<slug>/*.md` | Intent, research, requirements, decisions, task plan, and assurance project records. |
| `artifacts/changes/<slug>/events/` and `verification/` | Bundle-local lifecycle traces and skill verification records. |
| `.git/slipway/runtime/changes/<slug>/evidence/**` | Git-local runtime task evidence consumed by wave execution and freshness diagnostics. |

`status`, `validate`, and `next` recompute readiness without mutating state. `run` is the primary governed execution surface: it advances until a skill, blocker, checkpoint, or done-ready state is surfaced.

## First Actions

1. Install or build the CLI using [Installation](installation.md).
2. Initialize a repository with `slipway init` and optional AI-tool adapters.
3. Create a governed change with `slipway new`.
4. Use `slipway next --json` for a read-only handoff or `slipway run --json` to advance.
5. Close only after review, goal verification, and final closeout evidence pass.

## Design Boundaries

The [Design Philosophy](design.md#design-boundaries) page describes Slipway's authority, lifecycle, adapter, installation, and evidence boundaries. Use it to understand why Slipway keeps a small local governance kernel instead of becoming a hosted platform, project tracker, or host-specific agent workflow.
