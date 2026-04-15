---
skill_id: plan-authoring
domain: intake
function: turn requirements into bounded, auditable implementation tasks
tier: T1
primary_attachment: procedure
summary: "Use when drafting or auditing a plan bundle. Triggers on plan-audit host or on plan-file edits that require bounded execution-ready tasks."
trigger_signals:
  - host: plan-audit
    reason: "Plan audit host active; enforce bounded-task and execution-contract shape"
  - changed_files_include: ["docs/plans/*.md", "docs/plans/**/*.md"]
    reason: "Plan bundle touched; plan-authoring procedure applies"
evidence_contract: artifact
hydrate_references:
  - name: plan-document-review-prompt.md
    reason: "Reviewer prompt for auditing plan documents before execution"
bindings:
  - type: host-embedded
    target: plan-audit
    attachment: procedure
  - type: host-embedded
    target: plan-audit
    attachment: checklist
  - type: export-only
    target: using-slipway-catalog
    attachment: procedure
---

# Plan Authoring

```
IRON LAW: PLANS ARE BOUNDED, AUDITABLE TASKS — NOT WISH-LISTS
```

## Purpose
Produce a plan bundle whose tasks can be executed, reviewed, and verified
without further clarification. Absorbed from the `writing-plans`,
`workflow-patterns`, and `executing-plans` lineage.

## Procedure
1. Anchor to the approved scope summary; refuse to plan beyond it.
2. Split by *bounded outcome*, not by *file*. Each task ends in a testable
   signal (verdict, artifact, or checklist).
3. Sequence deliveries so each step leaves a reviewable PR. Avoid half-states
   that break the kernel between batches.
4. For guardrail-domain work, require the task to name a RED test plan.
5. Record execution-contract details next to the task: owner, blast radius,
   fallback, rollout batch.

## Checklist
- [ ] Every task lists concrete file/module scope.
- [ ] Every task names its evidence shape (verdict / artifact / checklist).
- [ ] Rollout is batched into reviewable PRs; inter-batch gate is explicit.
- [ ] No task introduces a second progression kernel.
- [ ] Non-goals are enumerated — at least scope boundary + rollout boundary.

## Anti-patterns
- Plans that mix "research" and "implementation" in the same task.
- Tasks named after files (`refactor X.go`) rather than outcomes.
- Rollout batches that silently assume all prior work merged.
- Optional backwards-compat shims for features never shipped.

## Failure handling
- Missing scope → bounce to `scope-clarification`, do not author tasks.
- Guardrail domain without test plan → flag blocker `guardrail_missing_tdd`.
