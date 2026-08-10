# Slipway documentation

Slipway wraps an AI coding host in a small, user-controlled workflow. Start with the task you want to accomplish; use the protocol and architecture pages only when you are integrating or maintaining Slipway.

[简体中文](../zh/index.md) · [日本語](../ja/index.md)

## Get started

- [Start here](start-here.md) — build or install Slipway, add one host adapter, and run one task.
- [Installation](installation.md) — release compatibility, packages, source builds, upgrades, and removal.
- [Core concepts](explanation/concepts.md) — Run, Action, source, Objective, Change, and completion semantics.

## Guides

- [Idea-to-Run workflow](guides/idea-to-run-workflow.md) — route a rough idea, Objective, Change, or existing Run across Slipway's user-owned function boundaries.
- [GitHub Issues](guides/github-issues.md) — when to use an Objective or Change and how issue-backed Runs work.
- [Runs, recovery, and privacy](guides/runs-and-recovery.md) — inspect, stop, resume, retain, or remove a Run.
- [Machine protocol v2 tutorial](guides/machine-protocol-v2.md) — run a complete host integration lifecycle with strict Outcomes.

## Reference

- [Commands](reference/commands.md) — the seven user commands, the `protocol` operations, and their flags.
- [Host adapters](reference/adapters.md) — generated targets, invocation styles, and ownership safety.
- [Machine protocol](reference/machine-protocol.md) — versioned JSON for host integrations.

## Maintainers

- [Architecture](explanation/architecture.md) — process boundaries, packages, storage, and trust boundaries.
- [Development reference](contributing.md) — repository layout and verification.
- [Contributing](../../CONTRIBUTING.md) — pull-request workflow.
- [Acceptance suite](../../acceptance/README.md) — executable and manual behavior checks.
- [Architecture decisions](../../adr/README.md) — historical rationale, kept outside user documentation.

The three language trees describe the same product. Exact machine field shapes live in the language-neutral JSON schemas; no translation is treated as a separate product contract.
