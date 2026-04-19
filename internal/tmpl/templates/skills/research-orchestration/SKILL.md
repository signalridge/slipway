---
skill_id: research-orchestration
name: research-orchestration
description: "Use when governed discovery needs architecture, pattern, risk, and test-strategy coverage before planning. Triggers on discovery-required changes or unresolved technical unknowns during intake."
---

# Research Orchestration

```
IRON LAW: NO SCOPE CONFIRMATION WITHOUT STRUCTURED RESEARCH
```

Violating the letter of this rule is violating the spirit of this rule.

## Purpose
Ensure governed discovery has sufficient breadth across architecture, patterns,
risks, and test strategy before plan audit begins. This is the governed
research host skill; `codebase-mapping` remains the reusable technique that
supplies durable structural context.
Mitigates: insufficient discovery breadth before plan audit.

## When This Runs
Discovery-required governed changes only, at `S1_PLAN/research` substep.
`slipway next` returns `next_skill: research-orchestration`.

## Process

### 1. Read Context
Run `slipway next --json` and examine the governed change artifacts.
Read the `research.md` artifact for existing discovery notes.

### 2. Structured Research Dimensions
Investigate the following dimensions systematically:

**Architecture**
- Map affected modules, dependency chains, and integration boundaries
- Identify coupling points and blast radius
- Document architectural constraints and invariants

**Patterns**
- Search for existing patterns and conventions in the target areas
- Identify reusable abstractions and established idioms
- Note deviations from project conventions that the change may require

**Risks**
- Enumerate technical risks (data loss, performance, security, compatibility)
- Identify guardrail domains touched by the change
- Assess reversibility of proposed changes

**Test Strategy**
- Map existing test coverage for affected areas
- Identify test infrastructure requirements
- Propose verification approach for key acceptance criteria

### 2b. Approach Alternatives (Brainstorming)
After investigating the four research dimensions, propose **2-3 alternative approaches** for the change:
- For each approach: describe the design, list tradeoffs (complexity, risk, reversibility, performance)
- Recommend one approach with a clear rationale
- Present alternatives to the user and wait for their selection before proceeding

This ensures the user makes an informed decision rather than the agent choosing silently. The selected approach must be reflected in `research.md` under `## Alternatives Considered`, and the locked decision recorded in `decision.md` under `## Selected Approach`.

### 3. Codebase Mapping (SHOULD)
If `input_context.codebase_map_dir` already contains documents, read at least:
- `ARCHITECTURE.md`
- `TESTING.md`
- `CONCERNS.md`

If the durable codebase map is missing or stale, run the `codebase-mapping` technique skill first and write the documents into `input_context.codebase_map_dir`.
This provides reusable structural context for research findings instead of one-off chat notes.

### 4. Write Verification
```yaml
# Write to: artifacts/changes/{slug}/verification/research-orchestration.yaml
verdict: pass
blockers: []
timestamp: "<ISO-8601-UTC>"
run_version: 0
references: []
notes: |
  <verification notes>
```

### 5. Surface Findings
Present structured research summary to user organized by dimension using the following format:

#### Structured Research Output Format
The research output MUST follow this structure so that `plan-audit` can validate it:

```markdown
## Research Findings

### Architecture
- Affected modules: [list with file paths]
- Dependency chains: [module → module relationships]
- Blast radius: [scope of impact]
- Constraints: [architectural invariants that must be preserved]

### Patterns
- Existing conventions: [patterns found in target areas]
- Reusable abstractions: [interfaces, helpers, utilities to leverage]
- Convention deviations: [where the change may need to break convention]

### Risks
- Technical risks: [enumerated with severity: high/medium/low]
- Guardrail domains: [any high-risk domains touched]
- Reversibility: [can the change be rolled back safely?]

### Test Strategy
- Existing coverage: [what's covered, what's not]
- Infrastructure needs: [new test helpers, fixtures, mocks needed]
- Verification approach: [how to verify each acceptance criterion]

### Unknowns Resolved
- [Entry-time unknown or open question] → [Resolution or finding]

### Remaining Questions
- [Questions that plan-audit must address]
```

This format enables `plan-audit` to validate that all research dimensions have findings and that entry-time unknowns have been addressed.

After presenting findings, <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway next` until the user approves.</HARD-GATE>

After confirmation: `slipway next`

## Required Coverage
All four research dimensions must have findings:
1. Architecture analysis with concrete module/file references
2. Pattern inventory with code examples
3. Risk enumeration with severity assessment
4. Test strategy with coverage gaps identified

## Failure Handling
- If any research dimension has no findings, set verdict to "fail" with specific dimension blockers.
- If codebase access is restricted, document the limitation and adjust scope.
- If findings reveal the change is larger than expected, note this for plan audit.

## Rationalization Red Flags
| Rationalization | Counter-rule |
|---|---|
| "The codebase is simple enough" | Simple codebases still have patterns and constraints worth documenting. |
| "Risk analysis is overkill for this" | Governed discovery exists because risk matters. Skip risk analysis and you skip its purpose. |
| "Tests can be figured out during implementation" | Test strategy informs scope. Unknown test needs create scope drift. |
| "I already know the architecture" | Document it anyway. Undocumented knowledge is unverifiable knowledge. |
| "Research is slowing us down" | Research prevents rework. Rework is slower than research. |

## DO NOT SKIP
1. All four research dimensions must have concrete findings (not placeholders).
2. Evidence-backed findings only (no "should be fine" or "probably safe").
3. Approach alternatives must be presented to user before proceeding.

## Step Declaration
Declare current step and expected output before executing each workflow step.
