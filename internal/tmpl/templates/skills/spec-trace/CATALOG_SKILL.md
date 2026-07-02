---
skill_id: spec-trace
domain: review-change-shape
function: bidirectional spec-to-code and code-to-spec trace review
tier: T1
primary_attachment: checklist
summary: "Use when tracing the approved plan and code in both directions (plan line to code, diff hunk back to plan line) to catch drift. Triggers on the spec-compliance-review stage, `slipway review` (auto-attached), or `slipway validate --focus spec-trace`."
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
contract; `slipway review` auto-attaches it (there is no `review --focus
spec-trace` selector — that is rejected), and `slipway validate --focus
spec-trace` exposes it as an explicit focus.

## Report schema
```yaml
verdict: pass | changes-requested | blocked
bottom_line: "<one-sentence summary>"
spec_to_code:
  - spec_item: "<quoted plan line>"
    status: covered | skipped | drift | ambiguous | uncheckable
    realized_by: "<path:line or test name>"
    reason: "<why this mapping is ambiguous or uncheckable>"
  - spec_item: "<quoted plan line>"
    status: skipped
    skipped_justification: "<reason reviewer accepted>"
code_to_spec:
  - diff_hunk: "<path:line-range>"
    status: covered | skipped | drift | ambiguous | uncheckable
    plan_item: "<quoted plan line>"
    reason: "<why this mapping is ambiguous or uncheckable>"
  - diff_hunk: "<path:line-range>"
    status: drift
    drift: "<why this is outside scope>"
coverage_gaps:
  - item: "<spec item or diff hunk>"
    status: ambiguous | uncheckable
    reason: "<why this mapping could not be verified>"
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
- Verdict `pass` when `ambiguous` or `uncheckable` rows remain unresolved or
  are missing from `coverage_gaps`.
