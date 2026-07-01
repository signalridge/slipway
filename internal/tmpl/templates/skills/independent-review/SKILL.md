---
skill_id: independent-review
domain: review-quality
function: workflow-owned S3 fresh-context code review with explicit verdict contract and reviewer-handoff discipline
tier: T1
primary_attachment: procedure
summary: "Use when a fresh-context S3 code review with an explicit verdict contract is needed. Triggers on the workflow-owned S3 review host or the `slipway review` command surface."
trigger_signals:
  - command: review
    reason: "review command invoked; attach independent-review report schema"
evidence_contract: verdict
bindings:
  - type: command-auto
    target: review
    attachment: report-schema
host_capabilities:
  - capability: subagent
    required: true
    fallback_modes:
      - manual_independent_review
      - same_context_degraded
    evidence_requirement: "record independent-review evidence from a fresh independent reviewer context"
    remediation: "Run independent-review in a host with subagent capability, or explicitly select manual_independent_review / same_context_degraded fallback and record fresh reviewer evidence with context_origin:stage=review=<handle> plus a fallback:<mode> reference when degraded."
---

# Independent Review

```
IRON LAW: REVIEW WITH FRESH CONTEXT, EXIT WITH A VERDICT
```

## Purpose
Perform code review as an independent reader. In S3 this runs as a
workflow-owned review peer dispatched through the configured `review` slot,
defaulting to native host dispatch when no slot is configured. If the directive
includes `engine_boundary`, honor it as Slipway's slot-level mutation/read-only
boundary, not as a provider capability description. When the directive carries
`session_instructions`, read it before dispatching and translate any described
model, backend/runtime (for example Codex or Claude), or tool intent into the
concrete parameters the selected `type`/`name` target accepts; Slipway does not
model these provider parameters. Do not reuse the author's
narration as your source of truth.

## Diff-scoped review

When a concrete diff target is in scope (pull request, commit range, or
changed-only selection), apply the diff-scoped rules below in addition to the
fresh-context discipline above. When no diff context is present (`review
--all`, or other full-review paths), skip this section entirely — the base
review contract stands on its own.

### Classify every finding
- **new** — finding is introduced by the diff under review. Blocking by default.
- **pre-existing** — finding existed before the diff and is unchanged. Report it, but do not block on it.
- **worsened** — finding existed before the diff, but the diff broadens its blast radius or weakens a mitigation. Treat it as blocking.

### Diff-scoped blocker policy
- Only **new** and **worsened** findings participate in the verdict.
  **pre-existing** findings never flip a verdict from pass to fail.
- Removed code in a "security", "CVE", or "fix" commit is blocking on
  inspection until git blame proves the removal is safe.
- Access-control removals (e.g. guard downgraded, validation deleted) are
  blocking until the diff author demonstrates an equivalent replacement.
- Report all three categories so ambient pre-existing risk stays visible
  without inflating the verdict.

### Evidence contract
Diff-scoped review preserves the `verdict` evidence contract. Emit the same
verdict record shape as the base review: explicit verdict, blocker list with
reproducible observations, and a reviewer-handoff summary. Do not silently
downgrade to an artifact-shaped record when operating on a diff.
