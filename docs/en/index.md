# Slipway documentation

Slipway adds a small, user-controlled workflow around an AI coding host. Start with the task you want to accomplish; use the protocol and architecture pages only when you are integrating or maintaining Slipway.

[简体中文](../zh/index.md) · [日本語](../ja/index.md)

## Get started

- [Start here](start-here.md) — build or install Slipway, add one host adapter, and run one task.
- [Installation](installation.md) — release compatibility, packages, source builds, upgrades, and removal.
- [Core concepts](explanation/concepts.md) — Run, Action, source, Objective, Change, and completion semantics.

## Guides

- [GitHub Issues](guides/github-issues.md) — when to use an Objective or Change and how issue-backed Runs work.
- [Runs, recovery, and privacy](guides/runs-and-recovery.md) — inspect, stop, resume, retain, or remove a Run.
- [Machine protocol v2 tutorial](guides/machine-protocol-v2.md) — run a complete host integration lifecycle with strict Outcomes.

## Reference

- [Commands](reference/commands.md) — the seven public CLI commands and their flags.
- [Host adapters](reference/adapters.md) — generated targets, invocation styles, and ownership safety.
- [Machine protocol](reference/machine-protocol.md) — versioned JSON for host integrations.

## Maintainers

- [Architecture](explanation/architecture.md) — process boundaries, packages, storage, and trust boundaries.
- [Development reference](contributing.md) — repository layout and verification.
- [Contributing](../../CONTRIBUTING.md) — pull-request workflow.
- [Acceptance suite](../../acceptance/README.md) — executable and manual behavior checks.
- [Architecture decisions](../../adr/README.md) — historical rationale, kept outside user documentation.

The three language trees describe the same product. Exact machine field shapes live in the language-neutral JSON schemas; no translation is treated as a separate product contract.
