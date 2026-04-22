---
skill_id: independent-review
domain: review-quality
function: fresh-context code review with explicit verdict contract and reviewer-handoff discipline
tier: T1
primary_attachment: procedure
summary: "Use when performing code review with a verdict contract. Triggers on review host or the review command surface."
trigger_signals:
  - host: ["spec-compliance-review", "code-quality-review"]
    reason: "Review host active; anchor fresh-context review discipline"
  - command: review
    reason: "review command invoked; attach independent-review procedure"
evidence_contract: verdict
bindings:
  - type: host-embedded
    target: spec-compliance-review
    attachment: procedure
  - type: host-embedded
    target: code-quality-review
    attachment: procedure
  - type: host-embedded
    target: code-quality-review
    attachment: checklist
  - type: command-auto
    target: review
    attachment: report-schema
---

# Independent Review

```
IRON LAW: REVIEW WITH FRESH CONTEXT, EXIT WITH A VERDICT
```

## Purpose
Perform code review as an independent reader. The attached procedure,
checklist, and verdict schema are the base contract. Do not reuse the author's
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
