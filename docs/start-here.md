# Start Here

This page is the shortest path from "I have a repo" to "Slipway is governing
one real change."

Slipway is built around a few durable project records:

| Slipway surface | What it does |
| --- | --- |
| Governed change | One bounded unit of work under `artifacts/changes/<slug>/`. |
| Codebase map | Shared repo context under `artifacts/codebase/` for brownfield work. |
| Task evidence | Runtime proof under `.git/slipway/runtime/changes/<slug>/evidence/`. |
| Review evidence | Fresh verification records that must match the current worktree. |
| AI adapters | Generated host files that route agents back to the Slipway CLI. |

The CLI is the authority. AI tools can help author artifacts and run stages, but
they should not invent lifecycle state or edit evidence by hand.

## Choose Your Path

| Situation | Start with |
| --- | --- |
| You are new to Slipway and want a small end-to-end run. | [First governed change](tutorials/first-governed-change.md) |
| You are adding Slipway to a repo that already has behavior. | [Onboarding an existing codebase](tutorials/onboarding-existing-codebase.md) |
| You need the install and adapter commands only. | [Install and refresh adapters](how-to/install-and-refresh-adapters.md) |
| A change is stuck, stale, or confusing. | [Recover and troubleshoot](how-to/recover-and-troubleshoot.md) |
| You are evaluating the design. | [Design](explanation/design.md) and [Workflow](explanation/workflow.md) |

For concrete adoption patterns, use
[Real-World Scenarios](real-world-scenarios.md).

## First Install

Pick the install path your platform supports. Common options:

```bash
brew install --cask signalridge/tap/slipway
go install github.com/signalridge/slipway@latest
```

Then confirm the binary is visible:

```bash
slipway --help
```

The full platform matrix, release archive paths, checksum verification, and
source-build instructions remain in [Installation](installation.md).

## Initialize A Repo

Run this from the repository root:

```bash
slipway init --tools codex
```

Use the tool IDs you actually use:

```bash
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`slipway init` writes `.slipway.yaml` and optional generated AI-tool adapters.
The adapters are convenience surfaces; the CLI remains authoritative.

## Start One Governed Change

For work that is more than a trivial edit, create a governed change:

```bash
slipway new "add a short usage note to README" --profile docs --preset standard
```

Inspect without mutating:

```bash
slipway status --json
slipway next --json --diagnostics
```

Advance only through Slipway-owned stages:

```bash
slipway run --json --diagnostics
```

If `run` returns a skill handoff, complete that handoff in your AI tool and
rerun the read-only command before continuing.

## If It Fails Closed

Fail-closed output is a feature. It means Slipway saw missing or stale proof and
named the next safe action.

Use this order:

```bash
slipway status --json
slipway validate --json
slipway next --json --diagnostics
slipway health --doctor --json
```

Then follow the named recovery command. Do not hand-edit `change.yaml`,
verification YAML, task evidence, or lifecycle timestamps. If evidence is stale,
rerun the owning stage, reviewer, or task evidence path so Slipway can re-derive
freshness from the current worktree.

## Keep Going

- Follow [First governed change](tutorials/first-governed-change.md) for a
  copy-pasteable first run.
- Follow [Onboarding an existing codebase](tutorials/onboarding-existing-codebase.md)
  when the repo already has conventions the agent must learn.
- Use [Commands](reference/commands.md) when you need exact command and JSON
  surface details.
