# Slipway documentation

Slipway is an explicitly invoked, issue-first but never issue-gated soft autopilot for AI coding. The English pages are non-normative summaries: the complete [Chinese product contract](../zh/reference/product-contract.md) and versioned [machine protocol schema](../reference/machine-protocol.schema.json) are implementation authorities.

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
            orient → clarify if needed → implement → review on observed diff → summarize
```

## Get started

- [Start here](start-here.md) — shortest path from install to one Run.
- [Installation](installation.md) — platform paths and adapter commands.
- [Product authority](reference/product-overview.md) — the four-axis model, six capabilities, seven commands.

## Reference

- [Issue workflow](reference/issue-workflow.md) — Objective/Change markers, labels, self-containment, GitHub limits, publication.
- [Commands](reference/commands.md) — public command and JSON surface.
- [Machine protocol](reference/machine-protocol.md) — versioned Action / Outcome contract and hidden operations.
- [Host adapters](reference/adapters.md) — ten hosts, six capabilities, ownership safety.
- [Windows rendering and durability](reference/windows-rendering-and-durability.md) — argv rendering and crash durability.
- [Acceptance and evidence](reference/acceptance-evidence.md) — evidence types and the scenario matrix.

## Explanation

- [Architecture](explanation/architecture.md) — package layout and dependency direction.
- [Runs and privacy](explanation/runs-and-privacy.md) — journal contents, retention, and the privacy promise.

## Decisions and scenarios

- [Architecture decisions](../decisions/0001-source-bundle-v2.md) — manifest-addressed source bundles.
- [Prompt scenarios](../../acceptance/README.md) — host-behavior evaluations.
