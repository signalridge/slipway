---
skill_id: coverage-analysis
domain: verification
function: evaluate test coverage against the change surface with a reproducible report
tier: T1
primary_attachment: checklist
summary: "Use when a change needs coverage evaluation. Triggers on validate command, goal-verification host, or coverage-related user text."
trigger_signals:
  - command: validate
    reason: "validate command invoked; coverage report applies"
  - host: goal-verification
    reason: "Verification host active; coverage is a verification input"
  - user_text_matches: ["coverage", "uncovered", "untested"]
    reason: "User text names coverage"
evidence_contract: verdict
bindings:
  - type: command-manual
    target: validate
    attachment: checklist
  - type: host-embedded
    target: goal-verification
    attachment: checklist
provenance_ref: provenance.yaml
---

# Coverage Analysis

```
IRON LAW: COVERAGE IS A DIAGNOSTIC, NOT A PROOF
```

## Purpose
Evaluate test coverage against the *change surface*, not against the whole
codebase. A coverage number without a change-surface denominator cannot tell
you whether the change is tested.

## Checklist
- [ ] Coverage measured with a pinned tool + version; command recorded.
- [ ] Denominator scoped to the change surface (new + modified lines) and
      reported separately from whole-codebase numbers.
- [ ] Uncovered new/modified lines are enumerated with file:line.
- [ ] For each uncovered line, either add a test or justify the exclusion
      (unreachable, trivial wrapper, generated code).
- [ ] End-to-end coverage gaps are called out; unit-only coverage is flagged
      when the change crosses process boundaries.
- [ ] Delta vs baseline is reported; a coverage drop blocks the verdict.

## Anti-patterns
- Reporting whole-codebase coverage and calling the change tested.
- Excluding uncovered lines silently because "they're hard to test".
- Treating 100% line coverage as behavioral coverage.
