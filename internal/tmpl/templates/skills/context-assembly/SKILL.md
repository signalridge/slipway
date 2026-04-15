---
skill_id: context-assembly
domain: intake
function: assemble product, codebase, and risk context before planning or review
tier: T1
primary_attachment: procedure
summary: "Use when a task needs grounded context before planning or review. Triggers on research or plan-audit hosts, unclear context, or action-scoped hydration cues."
size_rationale: "Warn-band accepted: keeps explicit anti-patterns and checklist anchors so context handoff quality remains reviewable."
trigger_signals:
  - host: ["research-orchestration", "plan-audit"]
    reason: "Research or plan host active; assemble context first"
  - user_text_matches: ["context", "background", "how does this work"]
    reason: "User text asks for context"
evidence_contract: artifact
hydrate_references:
  - name: codebase-map.md
    reason: "Ground brownfield context before planning"
bindings:
  - type: host-embedded
    target: research-orchestration
    attachment: procedure
  - type: host-embedded
    target: plan-audit
    attachment: posture
  - type: technique-hint
    target: research-orchestration
    attachment: procedure
provenance_ref: provenance.yaml
---

# Context Assembly

```
IRON LAW: NO PLANNING WITHOUT GROUNDED CONTEXT
```

## Purpose
Assemble product intent, codebase structure, and risk context before planning
or review begins. This skill is a procedure + posture that rides on top of the
`research-orchestration` and `plan-audit` hosts. It does not replace the host
or change which skill runs next.

## Procedure
1. State the question in one line before reading anything.
2. Traverse from entry points outward: identify the seam the change touches,
   then walk callers and collaborators until the blast radius is bounded.
3. Record each claim with a file:line citation or command transcript.
4. Capture the risk surface: what fails silently, what is load-bearing, what
   is shared with other consumers.
5. Finish with a one-screen summary: intent, seams, risks, open questions.

## Posture
- Read code before reading narration; re-derive before trusting prior notes.
- Prefer concrete citations over paraphrase.
- Mark every assumption as assumption, not finding, until verified.

## Checklist
- [ ] Question is restated in one line at the top of the summary.
- [ ] At least one entry point is named with a file:line citation.
- [ ] Callers / collaborators enumerated to the blast radius boundary.
- [ ] Risk surface enumerates load-bearing invariants and shared consumers.
- [ ] Open questions separated from findings.

## Anti-patterns
| Rationalization | Counter-rule |
|---|---|
| "I already know this area" | Re-derive anyway; memory drifts faster than code. |
| "The plan will surface context later" | Planning without context produces brittle tasks. |
| "Summaries are overhead" | Summaries are the reviewable handoff. |
