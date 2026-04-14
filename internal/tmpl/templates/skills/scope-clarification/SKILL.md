---
skill_id: scope-clarification
domain: intake
function: converge intent and scope before planning begins
tier: T1
primary_attachment: posture
summary: "Use when user intent or scope is ambiguous before planning. Triggers on intake host, unclear acceptance, or open clarifying questions."
trigger_signals:
  - all_of:
      - host: intake-clarification
    reason: "Intake host active; anchor scope posture before questions"
  - user_text_matches: ["underspecified", "unclear scope", "ambiguous", "not sure what"]
    reason: "User text signals underspecified intent"
evidence_contract: checklist
bindings:
  - type: host-embedded
    target: intake-clarification
    attachment: posture
  - type: host-embedded
    target: intake-clarification
    attachment: checklist
  - type: technique-hint
    target: intake-clarification
    attachment: posture
provenance_ref: provenance.yaml
---

# Scope Clarification

```
IRON LAW: NO PLANNING WITHOUT BOUNDED SCOPE
```

## Purpose
Converge user intent into a bounded, auditable scope statement before any
planning begins. This skill is a posture + checklist that rides on top of the
governed `intake-clarification` host. It does not replace the host.

## Posture
1. Surface the unspoken assumption before asking the next question.
2. Prefer one well-targeted question over three scattered ones.
3. Abbreviate when the user signals "trivial" / "just testing" / "quick fix".
4. Do not move to `plan` until the approved summary names what is in and out.

## Checklist
- [ ] `## In Scope` lists concrete, code-level items (files or API surfaces).
- [ ] `## Out of Scope` names at least one explicit exclusion.
- [ ] `## Acceptance Signals` names at least one verifiable check.
- [ ] `## Approved Summary` was reviewed by the user before advancing.
- [ ] Any unresolved technical unknowns live under `## Open Questions`.

## Anti-patterns
| Rationalization | Counter-rule |
|---|---|
| "Scope is obvious from the title" | Name the boundaries the title omits. |
| "I'll tighten scope in review" | Intent drift starts here, not in review. |
| "The user will correct misunderstandings" | Later corrections are expensive. |
