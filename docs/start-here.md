# Start here

> English is a non-normative guide. The complete [Chinese product contract](zh/reference/product-contract.md) and [machine schema](reference/machine-protocol.schema.json) are implementation authorities.

Slipway coordinates an AI coding host only after explicit invocation. Work is issue-first, not issue-gated:

```text
Objective Issue (optional, never executable)
  └─ self-contained Change Issue
       └─ Run: orient → clarify if needed → implement → review on observed diff → summarize
```

A Change is the only issue-backed source and carries all effective Requirements. Use an Objective only for multiple independent deliveries. GitHub unavailable, sensitive, tiny, urgent, or intentionally untracked work can start ad-hoc.

## Install and start

```bash
go install github.com/signalridge/slipway@latest
cd your-git-repository
slipway install --tool claude
slipway run "add a CSV export to reports" --json
```

For an issue-bound Run, a trusted host fetches a strict GitHub Change envelope once:

```bash
slipway run "implement the bounded Change" --source-file /safe/temp/change-envelope.json --json
```

The marker-valid body is Level authority; title/label drift warns but does not gate. Read the [Issue workflow](reference/issue-workflow.md) before publication. A public Issue has no private switch; sensitive work may require a private repository, an appropriate security channel, or ad-hoc Run.

## User control

The host investigates repository facts before asking. Clarify follows the Matt Pocock `grill-me` discipline: one dependent human decision at a time with recommendation and trade-offs; complete requests ask zero questions; changed shared understanding is confirmed; wrap-up stops immediately and writes nothing. Every Action can be skipped without a reason. Runs can stop, resume, or be taken over. Review is read-only, reports Intent/Quality findings, and never starts a repair loop. `ended` means only that the automatic queue is empty.

Ten adapters generate six explicit capabilities; the CLI has seven public commands. Journals can include accepted Requirements, goals, answers, and command summaries. Review [Runs and privacy](explanation/runs-and-privacy.md), [Windows behavior](reference/windows-rendering-and-durability.md), and [acceptance evidence](reference/acceptance-evidence.md).
