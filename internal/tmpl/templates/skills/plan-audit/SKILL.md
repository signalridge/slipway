---
skill_id: plan-audit
name: slipway-plan-audit
description: "Use when validating that the governed artifact bundle is ready for execution. Triggers on post-authoring audit or whenever plan artifacts change materially."
---

# Plan Audit

```
IRON LAW: NO EXECUTION WITHOUT A VERIFIED, COMPLETE PLAN
```

## Purpose
Validate that the governed artifact bundle is ready for execution. This host is
the execution-readiness gate before implementation.

## Workflow Outline
Read the bundle and codebase-map context, audit artifacts/dimensions/task
shape, then write verification and wait for approval.

## Read Context
Run `slipway next --json`, locate the governed change bundle, and read the
durable codebase map when present.

If `input_context.codebase_map_dir` contains durable mapping documents, read at least:
- `ARCHITECTURE.md`
- `STRUCTURE.md`
- `TESTING.md`
- `CONCERNS.md`

Use them to verify task targets, blast radius, and test scaffolding. If the map
is absent, continue and record the missing brownfield context as an advisory
gap. If scope remains ambiguous after reading the bundle, stop and return to
intake clarification.

Before auditing task targets against the map, check
`input_context.codebase_map_status`. A `scaffold_only` or `baseline` status
means the documents are **non-durable** (template placeholders or CLI-detected
facts only); do not audit against them as if they were reviewed context. Record
the consume-time codebase-map advisory that `next`/`run` emits as an advisory
gap rather than treating the map as complete brownfield analysis.

A `populated` or `partial` status reflects content presence, not scope
relevance — a map authored for a prior change still reads `populated`. When
`next`/`run` emits the codebase-map relevance advisory (it fires for `populated`
and `partial`), judge whether the map matches this change's scope (affected
seams, blast radius, concerns) before auditing task targets against it, and
re-author stale sections in `artifacts/codebase` inline if it does not.
Populated is not the same as relevant. For a `partial` map, also inspect
`input_context.codebase_map_doc_states` and treat any per-doc `scaffold_only`,
`baseline`, or `missing` entry as non-durable for that document.

Use `slipway-coding-discipline` as the execution-shape bar: plans should stay
simple, goal-scoped, and sliced for surgical implementation.

## Validate Artifacts
Verify the **required artifact set** exists and is structurally valid:
- Required (all paths): `change.yaml`, `intent.md`, `requirements.md`, `tasks.md`
- Required on expanded / discovery paths: `decision.md`
- Required on standard/strict effective preset: `assurance.md`
- If `research.md` is present in the artifact bundle, include it in validation

Each artifact must be non-empty, structurally valid, free of stale code
references, and consistent with the stale-propagation graph. Tasks must have
clear acceptance criteria.

If `research.md` is present, also verify that:
- `## Alternatives Considered` is consistent with the selected approach in `decision.md`
- `## Canonical References` points to real docs/specs/code paths where possible
- `## Unknowns` and `## Assumptions` have been addressed or remain consistent with the plan

Any missing required artifact is a blocker. A single wave SHOULD contain no
more than 5 tasks; oversized waves are warnings, not blockers.

## Dimension Checklist (8D)
Explicitly check all eight dimensions and record blocker/warning IDs by
dimension:
1. **Coverage (Nyquist Check)**: every `REQ-*` SHOULD be covered by at least one task. Standard/strict uncovered requirements block; light uncovered requirements warn.
2. **Completeness**: each task has objective, `wave`, dependencies, and expected outputs.
3. **Dependency Integrity**: `depends_on` references valid earlier-wave tasks and has no cycles.
4. **Key Links**: each task names concrete `target_files` or evidence targets.
5. **Scope Control**: task targets stay inside declared scope.
6. **Context Compliance**: task metadata supports context-safe execution (`task_kind` where present).
7. **Test Coverage Mapping**: acceptance criteria map to automated checks; new scaffolding is an explicit dependency.
8. **Alternatives Considered**: required/present `decision.md` names at least 2 approaches, tradeoffs, and the selected approach.

On standard/strict, any failed dimension blocks. On light, only dimension #1
coverage failures downgrade to warnings.

## Sidecars
Apply `checklist-quality.md`: specificity, measurability, requirement-to-intent
traceability, edge cases, failure modes, alternatives, tradeoffs, and concrete
risks.

Audit tasks as execution units, not prose:
- split by bounded outcome, not file name alone
- require each task to name its evidence shape (`verdict` / `artifact` / `checklist`)
- require task acceptance criteria to be satisfiable during S2 execution; do
  not accept criteria that require future S3 review or S4 closeout evidence
  before the workflow can legally reach those states
- keep rollout in reviewable batches; do not admit half-states that break the kernel between batches
- for guardrail-domain work, require a RED test plan before execution
- keep non-goals explicit, including scope and rollout boundaries

## Write Verification
```yaml
# Write to: artifacts/changes/{slug}/verification/plan-audit.yaml
verdict: pass
blockers: []
timestamp: "<ISO-8601-UTC>"
run_version: 0
references: []
notes: |
  <verification notes>
```

## Present and Advance
Show audit results. <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway next` until the user approves.</HARD-GATE>

After confirmation: `slipway next`

## DO NOT SKIP
1. Confirm the verification file exists after audit.
2. Stop on audit blockers.
3. Re-run when governed artifacts change.

## Block If
- A required artifact is missing, empty, or structurally invalid.
- An 8D blocker applies for the effective preset.
- Scope is still ambiguous after reading the governed bundle.

See `references/audit-smells.md` for recurring audit rationalizations and
blocker patterns. Keep the inline host focused on artifact checks, not the full
smell catalog.
