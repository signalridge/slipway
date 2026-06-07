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

Check `input_context.codebase_map_status` before treating the map as durable
context. A `scaffold_only` or `baseline` status means the documents hold only
template placeholders or CLI-detected facts: they are **non-durable** and MUST
NOT be relied on as reviewed brownfield context. When `next`/`run` surfaces a
codebase-map advisory for a non-durable map, record it as a research finding and
refine the documents (file:line citations, module boundaries, change-specific
risks) before depending on them.

A `populated` or `partial` status means only that the documents differ from the
scaffold and the CLI baseline — it reflects content **presence, not scope
relevance**. A map authored for a prior change still reads `populated`. When
`next`/`run` surfaces the codebase-map relevance advisory (it fires for
`populated` and `partial`), judge whether the map's affected seams, blast radius,
and concerns match **this** change's scope; re-author any stale or out-of-scope
sections in `input_context.codebase_map_dir` inline (the assessment re-reads them
on every run) before relying on the map as reviewed context. Populated is not the
same as relevant.

For a `partial` map, also inspect `input_context.codebase_map_doc_states` and
treat any per-doc `scaffold_only`, `baseline`, or `missing` entry as non-durable
for that document.

If the codebase map is **missing** (`status: missing`), run the
`slipway-codebase-mapping` technique skill first and write the documents into
`input_context.codebase_map_dir`. For a `populated`/`partial` map that is
semantically stale, re-author the affected sections inline (above) — do not rerun
the technique skill, which only scaffolds a missing or non-durable set.

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
Write `research.md` using the artifact schema headings directly so `validate`
and the `G_scope` research gate (surfaced by `slipway next`/`run`) evaluate the
same structure the host asks you to produce:

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

Use this structure so the discovery gate can validate it. When `slipway run`
advances out of `S1_PLAN/research`, the `G_scope` gate checks that discovery
evidence exists and that the schema-required top-level headings (e.g.
`## Alternatives Considered`, `## Unknowns`, `## Assumptions`,
`## Canonical References`) are present and structurally valid — a missing/invalid
heading surfaces `research_structure_invalid`, missing discovery evidence
surfaces `missing_discovery_evidence`. The four research dimensions above
(Architecture, Patterns, Risks, Test Strategy) are required by this host for
decision-ready research but are not separately gated by the engine; keep them
complete so the later `S1_PLAN/audit` stage (`slipway-plan-audit`) can build on
solid discovery.

After presenting findings, <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway run` (the advancing command) until the user approves; `slipway next` is read-only preview and never advances.</HARD-GATE>

After confirmation, advance with `slipway run`.

## DO NOT SKIP
1. All four research dimensions must have concrete findings.
2. Evidence-backed findings only.
3. Approach alternatives must be shown before proceeding.

## Block If
- Any research dimension has no concrete findings.
- Alternatives are missing or the selected approach is not recorded.
- Codebase access limits make the research claims unverifiable.
