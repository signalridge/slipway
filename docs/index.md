# Slipway Documentation

Slipway is a governance CLI for AI-assisted software delivery. It gives local
AI-agent work a durable lifecycle, current-state authority, and evidence-backed
closeout inside the repository where the work happens.

You drive it in plain language. After a one-time `slipway init`, you tell your
AI agent what you want to build, and the generated adapter routes that request
through the governed lifecycle — no command sequence to memorize. Slipway stays
the authority on whether the change is actually done; the commands throughout
these docs are what the agent runs for you, and what you can run directly
whenever you want.

New users should begin with [Start Here](start-here.md). If you already know
your situation, use [Real-World Scenarios](real-world-scenarios.md) to choose a
workflow.

## Documentation Map

Documentation is organized by Diataxis so each page has one job.

| Section | Use it when | Pages |
| --- | --- | --- |
| Tutorials | You want a guided path with commands and prompts to copy. | [First governed change](tutorials/first-governed-change.md), [Onboarding an existing codebase](tutorials/onboarding-existing-codebase.md) |
| How-to | You need to complete a specific operational task. | [Install and refresh adapters](how-to/install-and-refresh-adapters.md), [Recover and troubleshoot](how-to/recover-and-troubleshoot.md) |
| Reference | You need authoritative command, adapter, and contributor facts. | [Commands](reference/commands.md), [AI tool adapters](reference/ai-tools.md), [Contributing](contributing.md) |
| Explanation | You want the design reasoning behind the workflow. | [Design](explanation/design.md), [Workflow](explanation/workflow.md) |

## Mental Model

Slipway keeps these core surfaces separate:

| Surface | Role |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | Current lifecycle and routing authority. |
| `artifacts/changes/<slug>/*.md` | Intent, research, requirements, decisions, task plan, and assurance records. |
| `artifacts/changes/<slug>/events/` and `verification/` | Bundle-local lifecycle traces and skill verification records. |
| `.git/slipway/runtime/changes/<slug>/evidence/**` | Git-local runtime task evidence consumed by wave execution and freshness diagnostics. |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | Optional per-change advisory continuation notes; not lifecycle authority, evidence, freshness, or a gate. |
| `artifacts/codebase/**` | Durable repo-scoped context for brownfield planning and review. |

`status`, `validate`, and `next` recompute readiness without mutating state.
`run` advances only until Slipway reaches a skill handoff, blocker, or
done-ready state.

Completion is deliberately hard to fake: every governed stage owns evidence that
the engine re-derives instead of trusting. Start with the tutorials when you want
to experience that loop, then use the reference pages when you need exact
command syntax.
