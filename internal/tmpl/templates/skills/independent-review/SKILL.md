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
provenance_ref: provenance.yaml
---

# Independent Review

```
IRON LAW: REVIEW WITH FRESH CONTEXT, EXIT WITH A VERDICT
```

## Purpose
Perform code review as an independent reader. Load context fresh; do not reuse
the author's narration. Exit with an explicit verdict and reviewer-handoff
record that the author can act on without re-asking.

## Anti-patterns
- "LGTM" with no traversal evidence.
- Blockers without reproducible observations.
- Nits buried in prose so the author misses them.
