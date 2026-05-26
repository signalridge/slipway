---
skill_id: research-orchestration
name: slipway-research-orchestration
description: "Use when governed discovery needs architecture, pattern, risk, and test-strategy coverage before planning. Triggers on discovery-required changes or unresolved technical unknowns during intake."
---

# Research Orchestration

```
IRON LAW: NO SCOPE CONFIRMATION WITHOUT STRUCTURED RESEARCH
```

## Purpose
Ensure governed discovery covers architecture, patterns, risks, and test
strategy before plan audit begins. `slipway-codebase-mapping` remains the
durable context technique; this host turns that context into a decision-ready
research bundle.

## When This Runs
Discovery-required governed changes only, at `S1_PLAN/research` substep.
`slipway next` returns `next_skill: research-orchestration`.

## Read Context
Run `slipway next --json` and examine the governed change artifacts.
Read the `research.md` artifact for existing discovery notes.

## Structured Research Dimensions
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

## Approach Alternatives
After investigating the four research dimensions, propose **2-3 alternative
approaches** for the change:
- For each approach: describe the design and list tradeoffs
- Recommend one approach with a clear rationale
- Present alternatives to the user and wait for their selection before proceeding

The selected approach must be reflected in `research.md` under
`## Alternatives Considered`, and the locked decision recorded in `decision.md`
under `## Selected Approach`.

## Codebase Mapping (SHOULD)
If `input_context.codebase_map_dir` already contains documents, read at least:
- `ARCHITECTURE.md`
- `TESTING.md`
- `CONCERNS.md`

If the durable codebase map is missing or stale, run the
`slipway-codebase-mapping` technique skill first and write the documents into
`input_context.codebase_map_dir`.

## Write Verification
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

## Surface Findings
Write `research.md` using the artifact schema headings directly so
`validate` and `slipway-plan-audit` evaluate the same structure the host asks
you to produce:

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

## Alternatives Considered
- [Approach 1]: [tradeoffs]
- [Approach 2]: [tradeoffs]
- Selected: [chosen direction and rationale]

## Unknowns
- Resolved: [entry-time unknown or open question] -> [resolution or finding]
- Remaining: [Questions that slipway-plan-audit must address] or "None"

## Assumptions
- [Assumption] - Evidence: [file path, command, or artifact reference]

## Canonical References
- `[file:path]` for each source used as planning authority
```

Use this structure so `slipway-plan-audit` can validate it. This format
enables `slipway-plan-audit` to validate that all research dimensions have
findings and that the schema-required top-level headings are present.

After presenting findings, <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway next` until the user approves.</HARD-GATE>

After confirmation: `slipway next`

## DO NOT SKIP
1. All four research dimensions must have concrete findings.
2. Evidence-backed findings only.
3. Approach alternatives must be shown before proceeding.

## Block If
- Any research dimension has no concrete findings.
- Alternatives are missing or the selected approach is not recorded.
- Codebase access limits make the research claims unverifiable.
