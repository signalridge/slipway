---
skill_id: multi-reviewer-calibration
domain: review-quality
function: calibrate multiple reviewers on severity and scope before merging findings
tier: T1
primary_attachment: procedure
summary: "Use when more than one reviewer will sign off. Triggers on code-quality-review host or review commands naming multi-reviewer intent."
trigger_signals:
  - host: code-quality-review
    reason: "Code-quality-review host active; calibrate reviewer severity"
  - command: review
    reason: "review command invoked; multi-reviewer calibration may apply"
  - user_text_matches: ["second reviewer", "adversarial", "panel review"]
    reason: "User text signals multiple reviewers"
evidence_contract: artifact
hydrate_references:
  - name: review-dimensions.md
    reason: "Dimensions for reviewer-severity calibration"
bindings:
  - type: host-embedded
    target: code-quality-review
    attachment: procedure
---

# Multi-Reviewer Calibration

```
IRON LAW: REVIEWERS SIGN OFF SEPARATELY, CALIBRATE BEFORE THE AUTHOR SEES IT
```

## Purpose
When more than one reviewer will sign off, calibrate their severity rubric
and scope before their findings reach the author. An adversarial reviewer is
only useful if their blockers share severity meaning with the primary
reviewer.

## Procedure
1. Reviewers read the change and produce findings independently; no
   cross-talk until each list is frozen.
2. The primary reviewer records severity rubric boundaries (what counts as
   `blocker` vs `major`) in one line before merging.
3. Compare finding lists. For every disagreement, resolve with the rubric, a
   reproducible observation, or a written deferral.
4. Merge into one verdict. Findings that survived calibration carry the
   primary reviewer's name; deferrals name the reviewer who raised them.
5. Deliver one verdict to the author, not two.

## Checklist
- [ ] Reviewers produced findings independently.
- [ ] Rubric boundary recorded before merge.
- [ ] Every disagreement has a resolution recorded (rubric / evidence /
      deferral).
- [ ] Author receives one merged verdict.
- [ ] Adversarial-only findings survive only with reproducible observation.

## Anti-patterns
- Reviewers trading notes before freezing their findings.
- Adversarial reviewer blocking on taste without reproducible observation.
- Two separate verdicts landing on the author without merge.
