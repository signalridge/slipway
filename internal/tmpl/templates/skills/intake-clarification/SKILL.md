---
skill_id: intake-clarification
name: intake-clarification
description: "Use when clarifying user intent, scope boundaries, and acceptance signals before planning. Triggers on governed intake or any change that cannot be planned from the initial request alone."
---

# Intake Clarification

```
IRON LAW: NO PLANNING WITHOUT CLEAR INTENT
```

## Purpose
Clarify user intent, define scope boundaries, and produce an approved summary
before planning begins. This is the governed intake host skill;
`scope-clarification` attaches the posture/checklist overlay that sharpens the
questions and boundary checks.
Mitigates: scope ambiguity, intent drift, over-scoping.

## When This Runs
All governed changes at S0_INTAKE/clarify or S0_INTAKE/research. `slipway next` returns `next_skill: intake-clarification`.

## Process

### 1. Read Context
Run `slipway next --json` to get the current change state and intent.md content.
Read the intent.md artifact in the governed bundle.

Carry the attached `scope-clarification` posture while reading:
- surface the unspoken assumption before the next question
- prefer one well-targeted question over scattered batches
- do not move to planning until the approved summary names both what is in and what is out

### 2. Assess Complexity
Check the `## Complexity Assessment` section in intent.md:
- **trivial**: Ask 1-2 quick confirmation questions, then proceed to confirmation
- **simple**: Ask 2-3 clarification questions focused on scope and acceptance
- **complex**: Ask 3-5 questions covering scope, constraints, dependencies, and risks
- **critical**: Ask 5+ questions, ensure guardrail domains are acknowledged, verify constraints

### 3. Clarification Loop (one question at a time)
For each clarification round:
1. Read the current state of intent.md
2. Identify the most important unresolved aspect
3. Ask ONE focused question to the user
4. Update intent.md with the user's response in the appropriate section
5. Repeat until all required sections have substantive content

**Required sections for advancement:**
- `## In Scope` — concrete, code-level items included (files, APIs, commands, or user-visible surfaces)
- `## Out of Scope` — at least one explicit exclusion
- `## Acceptance Signals` — at least one verifiable completion criterion
- `## Open Questions` — only unresolved technical unknowns that truly require research

**Abbreviation signals** — if the user says any of:
- "just testing", "trivial change", "quick fix", "that's it", "good enough"
→ Accept current scope, fill minimal sections, and move to confirmation.

### 4. Research Route (if needed)
If `## Open Questions` contains technical unknowns that cannot be resolved via clarification:
- Document them in `## Open Questions`
- The state machine will route to S0_INTAKE/research

### 5. Confirmation
Once scope is clear:
1. Write a concise summary in `## Approved Summary` that captures:
   - What the change does (1-2 sentences)
   - Key scope boundaries
   - Primary acceptance signal
2. Present the summary to the user
3. Confirm the summary names at least one explicit out-of-scope item and keeps any unresolved technical unknowns under `## Open Questions`.
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
After confirmation: `slipway next`

## DO NOT SKIP
1. At least one clarification question (even for trivial changes).
2. User confirmation of the approved summary.
3. Concrete `In Scope` / `Out of Scope` boundaries before planning.
4. Writing verification evidence.

## Rationalization Red Flags
| Rationalization | Counter-rule |
|---|---|
| "The description is clear enough" | Clear to you is not clear to the system. Verify with the user. |
| "This is too simple for clarification" | Even trivial changes need one confirmation. Named anti-pattern. |
| "I'll figure out scope during planning" | Scope must be defined before planning. Intent drift starts here. |
| "The user will correct me later" | Later corrections are expensive. Clarify now. |

## Scope Boundary Precision Rules
Scope items must be CONCRETE:
- BAD: "Improve error handling" (vague)
- GOOD: "Add typed errors in `internal/engine/` (6 files), replace `fmt.Errorf`"
- BAD: "Authentication improvements" (unbounded)
- GOOD: "Add JWT refresh in `internal/auth/token.go`, update login flow"

## Failure Handling
- If intent.md is missing or empty: create scaffold from description, then clarify.
- If user provides contradictory scope: note contradiction, ask for resolution.
- If user abandons clarification: do not advance. Keep state at S0_INTAKE/clarify.

## Step Declaration
Declare current step and expected output before executing each workflow step.
