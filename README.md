<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

# Slipway

**An explicitly invoked, issue-first but never issue-gated soft autopilot for AI coding.**

[简体中文](README.zh.md) · [日本語](README.ja.md) · [Documentation](docs/start-here.md)

</div>

> **English is a non-normative summary.** The complete [Chinese product contract](docs/zh/reference/product-contract.md) and versioned [machine protocol schema](docs/reference/machine-protocol.schema.json) are implementation authorities.

Slipway helps an AI coding host investigate a repository, clarify genuine human decisions, implement a bounded change, optionally review observed differences, recover interrupted runs, and report facts. The user starts it explicitly and can skip, stop, resume, or take over.

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one pinned, interruptible attempt)
            orient → clarify if needed → implement → review on observed diff → summarize
```

Requirements are temporary delivery contracts, not a permanent Spec. An Objective exists only for multiple independent deliveries; every Change is self-contained and does not inherit runtime requirements from its parent or comments. The first exact body marker is Level authority. Labels/title are warning-only projections; `ready-for-agent`, Issue state, Project fields, tests, and findings never gate a marker-valid Run.

## Quick start

```bash
go install github.com/signalridge/slipway@latest
cd your-repository
slipway install --tool claude

# Ad-hoc escape hatch: tiny, sensitive, urgent, offline, or no Issue by choice.
slipway run "add a CSV export to reports" --json

# Issue-bound: a trusted host fetches this strict Change envelope once.
slipway run "implement the bounded Change" \
  --source-file /safe/temp/change-envelope.json --json
```

The CLI does not call a model provider or hold a GitHub token. A trusted host fetches an untrusted Change body; the CLI validates it, pins exact accepted sections and revisions, then returns one versioned Action at a time. Amendments require an explicit current-candidate choice. Destructive work requires a one-shot exact-scope structured grant; natural-language approval never grants it.

Read the [Issue workflow](docs/reference/issue-workflow.md) for Objective/Change markers, exact Level/Kind labels, self-containment, `gh >= 2.94`/official REST fallback, same-host transfer handling, 100/50 limits, approved publication markers, and partial/ambiguous reconciliation.

## Six explicit host capabilities

Every supported adapter generates exactly:

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

Adapters support Claude Code, Codex, GitHub Copilot, Cursor, Kilo Code, Kiro, OpenCode, Pi, Qwen Code, and Windsurf. All capabilities require explicit invocation. Clarify preserves Matt Pocock's MIT-licensed `grill-me`/`grilling` discipline: investigate facts, walk dependent decisions one at a time with recommendations, confirm changed shared understanding, remain stateless, and stop immediately on wrap-up. There is no implicit clarification-document capability. Review is read-only and never repairs or opens a re-review loop.

## Seven public commands

```text
slipway install     install six host capabilities safely
slipway uninstall   remove only pristine managed files
slipway list        show adapter installation state
slipway doctor      diagnose adapters, Git/GitHub capability, and recovery
slipway run         start an ad-hoc or issue-bound Run
slipway status      list or inspect recoverable Runs
slipway stop        stop without deleting the journal
```

Hidden versioned `run submit/answer/skip/resume` operations are documented in the [machine protocol](docs/reference/machine-protocol.md). `ended` means only that the automatic Action queue is empty; Slipway does not certify correctness, delivery, deployment, release readiness, or absence of findings.

## Journal and privacy

Recovery authority lives at `.git/slipway/runs/<run-id>/journal.jsonl`; `run.json` is replaceable projection and `run.lock` only serializes journal mutation. A journal may contain accepted Requirements, goals, user answers, and truthful command summaries. Slipway does not promise a secret-free journal: it avoids raw Issue bodies/comments, tokens, environment dumps, full transcripts, and hidden reasoning, and redacts recognized credential values while preserving command identity. Unix modes and Windows current-user ACL intent have root/admin, backup, malware, inherited-ACL, and same-account limitations.

Deleting a run directory removes recovery capability only. It is not secure erase, backup purge, or key destruction. Read [Runs and privacy](docs/explanation/runs-and-privacy.md), [Windows behavior](docs/reference/windows-rendering-and-durability.md), and the honest [acceptance evidence matrix](tests/acceptance/README.md).

Slipway is distributed under the [BSD 3-Clause License](LICENSE).
