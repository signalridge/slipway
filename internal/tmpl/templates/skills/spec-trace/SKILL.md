---
skill_id: spec-trace
domain: review-change-shape
function: bidirectional spec-to-code and code-to-spec trace review
tier: T1
primary_attachment: checklist
summary: "Use when verifying that implementation mirrors the approved plan. Triggers on spec-compliance host or validate/review commands."
trigger_signals:
  - host: spec-compliance-review
    reason: "Spec-compliance host active; enforce spec trace"
  - command: ["validate", "review"]
    reason: "Validation or review path; run spec trace"
evidence_contract: verdict
bindings:
  - type: host-embedded
    target: spec-compliance-review
    attachment: checklist
  - type: command-auto
    target: validate
    attachment: checklist
  - type: command-auto
    target: review
    attachment: report-schema
---

# Spec Trace

```
IRON LAW: EVERY SPEC LINE MAPS TO CODE; EVERY CODE CHANGE MAPS TO SPEC
```

## Purpose
Verify that the implementation mirrors the approved plan in both directions.
Spec-to-code proves every promise was kept; code-to-spec proves no unapproved
scope crept in. Either direction alone is insufficient. This is the attached
checklist/report-schema skill used by `spec-compliance-review`; it supplies the
trace edges that the host review must cite.

## Report schema
```yaml
verdict: pass | changes-requested | blocked
bottom_line: "<one-sentence summary>"
spec_to_code:
  - spec_item: "<quoted plan line>"
    realized_by: "<path:line or test name>"
  - spec_item: "<quoted plan line>"
    skipped_justification: "<reason reviewer accepted>"
code_to_spec:
  - diff_hunk: "<path:line-range>"
    plan_item: "<quoted plan line>"
  - diff_hunk: "<path:line-range>"
    drift: "<why this is outside scope>"
```

## Anti-patterns
- "Plan was followed" without per-item citations.
- Diff hunks accepted as "refactor" without a plan line to cite.
- Verdict `pass` when at least one spec item is skipped without justification.
