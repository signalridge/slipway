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
Verify the approved plan in both directions: plan line to code, and diff hunk
back to plan line. `spec-compliance-review` uses this as its attached trace
contract, and `review` / `validate --focus spec-trace` keep it public.

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
scope_contract:
  status: pass | fail | not_applicable
  reference: "scope_contract:pass or scope_contract:fail:<reason>"
  out_of_scope_files: ["<path>"]
```

## Anti-patterns
- "Plan was followed" without per-item citations.
- Diff hunks accepted as "refactor" without a plan line to cite.
- Changed files accepted despite `scope_contract:fail:<reason>` evidence.
- Verdict `pass` when a skipped spec item has no justification.
