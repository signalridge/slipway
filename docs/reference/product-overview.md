# Product overview (non-normative)

> **Non-normative summary.** The complete [Chinese product contract](../zh/reference/product-contract.md) and versioned [machine protocol schema](machine-protocol.schema.json) are the implementation authorities. This page is navigation, not a second specification.

Slipway is an explicitly invoked, issue-first but never issue-gated soft autopilot for AI coding:

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
```

Requirements are temporary delivery contracts. An Objective groups outcomes that need multiple independent Changes; decomposition copies every applicable requirement and constraint into each child. A Change has one independently deliverable result and never inherits runtime requirements from its parent or ordinary discussion comments; its Issue-body manifest explicitly publishes independently addressable normative chapter comments. An ad-hoc Run remains available when GitHub is unavailable, the work is sensitive or tiny, or the user declines an Issue.

The four independent axes are Level (`objective|change`), Kind (`feature|bug|refactor|maintenance|research|docs`), ordered manifest-addressed Requirements chapters with five required semantic roles, and human/external Status. The exact first body marker is Level authority. Titles, labels, `ready-for-agent`, Project fields, test results, findings, and Issue state do not gate a marker-valid Run.

Ten adapters expose exactly six explicitly invoked capabilities: run, clarify, propose, decompose, implement, and read-only review. The CLI exposes exactly seven public commands: install, uninstall, list, doctor, run, status, and stop. A Run advances one versioned Action at a time and may be skipped, stopped, resumed, or taken over. `ended` means only that its automatic queue is empty.

Hosts attest GitHub fetches; Issue content remains untrusted. Manifest-referenced chapter bytes and domain-separated revisions are pinned locally, amendments require an explicit current candidate choice, and destructive authority is one-shot and exact-scope. Publication uses approved operation/item UUID markers and reconciliation because GitHub does not provide exactly-once Issue creation or body CAS. Review reports findings without editing or opening a repair loop.

The append-only journal in the Git common directory is recovery authority. Private per-Run material blobs can contain accepted Requirements, while the journal contains their catalog plus goals, answers, and command summaries; the complete Run directory must be treated as sensitive local data. Slipway minimizes and redacts recognized credentials but does not promise a secret-free journal. Deleting a run directory removes recovery capability, not backups or securely erased bytes.

Continue with [Issue workflow](issue-workflow.md), [commands](commands.md), [machine protocol](machine-protocol.md), [Windows behavior](windows-rendering-and-durability.md), and [acceptance evidence](acceptance-evidence.md).
