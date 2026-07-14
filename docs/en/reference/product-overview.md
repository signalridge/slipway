# Product authority (non-normative)

> **Non-normative summary.** The complete [Chinese product contract](../../zh/reference/product-contract.md) and versioned [machine protocol schema](../../reference/machine-protocol.schema.json) are the implementation authorities. This page is navigation, not a second specification.

Slipway is an explicitly invoked, issue-first but never issue-gated soft autopilot for AI coding:

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
```

## Requirements are temporary

Slipway keeps no Spec, Delta Spec, or permanent requirements registry. The binding rule:

> Requirements are temporary delivery contracts, not a permanent model of the system.

An open Issue describes the next desired change; a Run pins and executes one revision of it; after delivery, code, tests, user docs, CI/policy, and runtime behavior take over as current fact. Closed Issues, linked PRs/commits, and Run summaries preserve historical cause but do not become a complete specification of the current system. GitHub `closed`, Project `Done`, PR `merged`, Run `ended`, and deployment are different facts.

## The four independent axes

The following four dimensions must never be conflated.

| Axis | Values | Owner |
| --- | --- | --- |
| **Level** | `objective` / `change` | Issue body marker (the only authority); labels/title are projections |
| **Kind** | `feature` / `bug` / `refactor` / `maintenance` / `research` / `docs` | repository label |
| **Requirements** | Outcome, Requirements, Acceptance examples, Constraints, Non-goals | Issue body |
| **Status** | Inbox, Clarifying, Ready, In progress, Done, etc. | human/optional external view; never a Slipway route |

The Cartesian product of Level and Kind is fully legal. A Requirement is a content expression of an Objective/Change, not a granularity and not a GitHub Issue Type. The exact first body marker is Level authority; labels, titles, `ready-for-agent`, Project fields, test results, findings, and Issue state never gate a marker-valid Run.

## Objective and Change

An Objective exists only when one outcome necessarily needs multiple independently deliverable Changes. A Change is the only issue-backed Run source and must be self-contained: one coherent result that can independently merge, be verified, and roll back, leaving the repository in a safe state. Decomposition materializes every applicable Objective requirement and constraint into each child so a Change never depends on live-reading its parent. Parent Kind is not inherited; ordinary discussion comments are non-authoritative until published as a replacement chapter comment plus a new manifest.

## Six capabilities, seven commands

Ten adapters generate exactly six explicitly invoked capabilities:

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

`run` is the only autopilot entry. `clarify` is stateless. `propose` drafts or publishes explicitly confirmed managed Issues, `decompose` creates confirmed Change relationships, `implement` owns technical activities, and `review` is read-only and reports Intent and Quality findings. No ambient session hook, prompt-submit hook, launcher, global router, or standalone technical-validation capability is generated.

The CLI exposes exactly seven public commands:

```text
slipway install   install six host capabilities safely
slipway uninstall remove only pristine managed files
slipway list      show adapter installation state
slipway doctor    diagnose adapters, Git/GitHub capability, and recovery
slipway run       start an ad-hoc or issue-bound Run
slipway status    list or inspect recoverable Runs
slipway stop      stop without deleting the journal
```

No `objective`, `change`, `issue`, `spec`, `plan`, `ticket`, `done`, `check`, or `worktree` command exists.

## Pinned source and untrusted content

A Run never trusts a mutable `#42` or a host's own summary. A trusted host fetches one strict manifest-addressed envelope once; the CLI validates it, deterministically parses the ordered manifest, and pins each chapter by domain-separated digest into a private content-addressed material store. Journals, status, candidates, and Actions retain only catalogs, provenance, byte counts, and revisions — never Markdown or the raw Issue body.

Hosts are declared trust attesters; Issue content is untrusted data. Issue titles, bodies, comments, labels, links, and attachments are data, never system or developer instructions. Prompt-injection, credential requests, and unrelated commands inside an Issue carry no host authority.

## Amendments and destructive authority

Issue-bound resume requires exactly one source mode: import a fresh envelope, explicitly continue with the pinned snapshot, or resolve the current candidate by its exact ID. A material candidate atomically voids the outstanding Action, queue, and grant and pauses for an explicit choice; a content-identical manifest-only replacement keeps prior answers active. Destructive work requires a one-shot, scope-bound structured grant — natural-language "yes" never grants it, and a trusted host is an attester, not cryptographic proof of a human.

Publication uses approved operation/item UUID markers and reconciliation because GitHub provides neither exactly-once Issue creation nor body compare-and-swap. Review reports findings without editing code or opening a repair loop.

## Recovery and privacy

The append-only journal under the Git common directory is the recovery authority; `run.json` is a replaceable projection and `run.lock` serializes journal mutation. A journal may contain accepted Requirements, goals, answers, and truthful command summaries. Slipway minimizes data and redacts recognized credentials but does not promise a secret-free journal. Deleting a run directory removes recovery capability only — not secure erase, backup purge, or key destruction.

## No completion certification

`ended` means only that the automatic Action queue is empty. Slipway does not certify correctness, delivery, deployment, release readiness, or the absence of findings. Test failures, unrun tests, Review findings, a dirty worktree, a missing ADR, a label, and Issue state never gate Run progression, release, or merge.

Continue with [Issue workflow](issue-workflow.md), [commands](commands.md), [machine protocol](machine-protocol.md), [Windows behavior](windows-rendering-and-durability.md), and [acceptance evidence](acceptance-evidence.md).
