# Command Reference

All routed commands support `--json` when structured output is useful.

## Core Lifecycle

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway new [description]` | mutation | Create a governed change starting at intake. |
| `slipway next` | query | Inspect the next actionable skill or blocker without advancing state. |
| `slipway run` | mutation | Advance until a skill, blocker, checkpoint, or done-ready outcome is surfaced. |
| `slipway status` | query | Show lifecycle state, blockers, progress, and next actions. |
| `slipway done` | mutation | Finalize a done-ready change and archive it. |

## Creation Options

```bash
slipway new "add install docs" --preset standard
slipway new "docs-only change" --profile docs
slipway new --from-doc docs/installation.md "refresh install docs"
slipway new "small fix" --trivial
```

Presets control gate strictness: `light`, `standard`, or `strict`.

Workflow profiles shape checks: `code`, `docs`, `research`, `config`, or `meta`.

## Situational Commands

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway init` | mutation | Initialize `.slipway.yaml` and optional AI-tool adapters. |
| `slipway preset <level>` | mutation | Confirm or change the active change preset. |
| `slipway validate` | query | Recompute evidence and gate readiness without advancing. |
| `slipway review` | mutation | Run explicit artifact-code alignment review from execution onward. |
| `slipway checkpoint` | mutation | Pause execution for a task-level human response. |
| `slipway pivot` | mutation | Reroute or rescope an active change from execution onward. |
| `slipway abort` | mutation | Abort the active execution session without archiving the change. |
| `slipway cancel` | mutation | Cancel an active change and archive terminal state. |
| `slipway repair` | mutation | Run bounded local integrity repairs. |

## Diagnostics

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway learn --preview` | query | Preview governance learning proposals from lifecycle evidence. |
| `slipway stats` | query | Show repo-wide governance freshness and workflow statistics. |
| `slipway health` | query | Show repo-local integrity and repairability findings. |
| `slipway codebase-map` | mutation | Create or refresh advisory repo-scoped context under `artifacts/codebase/`. |

## Useful JSON Invocations

```bash
slipway new --json "refresh docs"
slipway next --json --diagnostics
slipway run --json --diagnostics
slipway status --json
slipway validate --json
slipway health --doctor --json
```

`next --json` includes `next_skill.name` for AI-tool handoff. The host tool derives the local `SKILL.md` path from its own adapter conventions.

## Resume And Checkpoints

If execution pauses on a checkpoint:

```bash
slipway status --json
slipway run --resume-response "approved"
```

If an execution session is resumable:

```bash
slipway run --resume --json
```

Use `health --doctor` before repair or resume when state looks interrupted or inconsistent.
