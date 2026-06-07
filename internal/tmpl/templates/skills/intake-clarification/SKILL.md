---
skill_id: intake-clarification
name: slipway-intake-clarification
description: "Use when clarifying user intent, scope boundaries, and acceptance signals before planning. Triggers on governed intake or any change that cannot be planned from the initial request alone."
---

# Intake Clarification

```
IRON LAW: NO PLANNING WITHOUT CLEAR INTENT
```

## Purpose
Clarify user intent, define scope boundaries, and produce an approved summary
before planning begins. Mitigates scope ambiguity, intent drift, and
over-scoping.

## Process

### 1. Read Context
Use `slipway next --json` for the current state and governed bundle path, then
read `intent.md`.

Carry this scope posture while reading:
- surface the unspoken assumption before the next question
- prefer one well-targeted question over scattered batches
- do not move to planning until the approved summary names both what is in and what is out

### 2. Assess Complexity
Use `## Complexity Assessment` to set question depth:
- **trivial**: 1-2 quick confirmation questions
- **simple**: 2-3 scope and acceptance questions
- **complex**: 3-5 scope, constraints, dependency, and risk questions
- **critical**: 5+ questions plus guardrail-domain and constraint checks

### 3. Clarification Loop (one question at a time)
For each clarification round:
1. Read the current state of intent.md
2. Identify the most important unresolved aspect
3. Ask ONE focused question to the user
4. Update intent.md with the user's response in the appropriate section
5. Repeat until all required sections have substantive content

**Required advancement sections:**
- `## In Scope`: concrete files, APIs, commands, or user-visible surfaces
- `## Out of Scope`: at least one explicit exclusion
- `## Acceptance Signals`: at least one verifiable check, not a hope
- `## Open Questions`: a `- [ ]` checklist of technical unknowns that truly
  require research; mark `- [x]` once resolved. You own this semantic judgment —
  the engine blocks only on an unchecked `- [ ]`, so a real unknown left as prose
  will NOT route to research. Leave the section empty (or `None`) when there are none.
- `## Approved Summary`: reviewed with the user before advancement

If the user says "just testing", "trivial change", "quick fix", "that's it",
or "good enough", accept current scope, fill minimal sections, and move to
confirmation.

### 4. Research Route (if needed)
If a technical unknown cannot be resolved via clarification:
- Record it as an unchecked `- [ ]` item under `## Open Questions` (one per unknown)
- `slipway run` then routes to S0_INTAKE/research; resolve it by marking `- [x]`
  (or removing it) once research answers it

### 5. Confirmation
Once scope is clear:
1. Write a concise summary in `## Approved Summary` that captures:
   - what the change does
   - key scope boundaries
   - primary acceptance signal
2. Present the summary to the user
3. Confirm it names at least one out-of-scope item and keeps unresolved technical unknowns under `## Open Questions`.
4. <HARD-GATE>Wait for explicit user confirmation before writing the approved summary.</HARD-GATE>

### 6. Write Verification
```yaml
# Write to: artifacts/changes/{slug}/verification/intake-clarification.yaml
verdict: pass
blockers: []
timestamp: "<ISO-8601-UTC>"
run_version: 0
references: []
notes: |
  <verification notes>
```

### 7. Advance
After confirmation, advance with `slipway run` (`slipway next` is read-only and does not advance; use it only to preview the next skill once `run` has advanced).

## DO NOT SKIP
1. At least one clarification question (even for trivial changes).
2. User confirmation of the approved summary.
3. Concrete `In Scope` / `Out of Scope` boundaries before planning.
4. Writing verification evidence.

## Scope Boundary Precision Rules
Scope items must be concrete:
- Bad: "Improve error handling"
- Good: "Add typed errors in `internal/engine/` (6 files), replace `fmt.Errorf`"
- Bad: "Authentication improvements"
- Good: "Add JWT refresh in `internal/auth/token.go`, update login flow"

## Failure Handling
- If intent.md is missing or empty: create scaffold from description, then clarify.
- If user provides contradictory scope: note contradiction, ask for resolution.
- If user abandons clarification: do not advance. Keep state at S0_INTAKE/clarify.
