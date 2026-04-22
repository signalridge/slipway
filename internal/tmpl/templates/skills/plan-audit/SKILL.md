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
Validate that the governed artifact bundle is ready for execution. This host
owns the execution-readiness gate before governed implementation begins.

## Workflow Outline
1. Read the governed bundle and durable codebase-map context.
2. Audit artifacts, dimensions, and task shape.
3. Write verification, surface blockers or warnings, and wait for approval.

## When This Runs
Before wave execution begins. `slipway next` returns `next_skill: plan-audit`.

## Read Context
Run `slipway next --json` and locate the governed change bundle.

If `input_context.codebase_map_dir` contains durable mapping documents, read at least:
- `ARCHITECTURE.md`
- `STRUCTURE.md`
- `TESTING.md`
- `CONCERNS.md`

Use them to verify task targets, blast radius assumptions, and test scaffolding
needs. If the map is absent, continue, but call out the missing brownfield
context as an advisory gap.

If the approved scope is still ambiguous after reading the bundle, stop and
return to intake clarification instead of inventing task boundaries during
audit.

Use `slipway-coding-discipline` as the execution-shape bar: plans should stay
simple, goal-scoped, and sliced for surgical implementation rather than broad
"touch everything" waves.

## Validate Artifacts
Verify the **required artifact set** exists and is structurally valid:
- Required (all paths): `change.yaml`, `intent.md`, `requirements.md`, `tasks.md`
- Required on expanded / discovery paths: `decision.md`
- Required on standard/strict effective preset: `assurance.md`
- If `research.md` is present in the artifact bundle, include it in validation

Each artifact must be checked for:
- Existence and non-empty content
- Structural validity (required sections present)
- Tasks have clear acceptance criteria
- No stale references to outdated code
- Stale propagation graph is clean

If `research.md` is present, also verify that:
- `## Alternatives Considered` is consistent with the selected approach in `decision.md`
- `## Canonical References` points to real docs/specs/code paths where possible
- `## Unknowns` and `## Assumptions` have been addressed or remain consistent with the plan

Any missing artifact from the required set is a blocker.

### Plan Size Advisory
A single wave SHOULD contain no more than 5 tasks. Oversized waves are a
warning, not a blocker, but they should be called out explicitly.

## Dimension Checklist (8D)
The plan audit MUST explicitly check all eight dimensions:
1. **Coverage (Nyquist Check)**: every requirement in `requirements.md` SHOULD have at least one task in `tasks.md` listing the same `REQ-*` ID in `covers`. On **standard/strict**, uncovered requirements are blockers. On **light**, they are warnings.
2. **Completeness**: each task has objective, explicit `wave`, dependencies, and expected outputs.
3. **Dependency Integrity**: `depends_on` references valid tasks, contains no cycles, and only points to earlier waves.
4. **Key Links**: each task links to concrete `target_files` or evidence targets.
5. **Scope Control**: task targets stay within declared scope boundaries.
6. **Context Compliance**: task metadata supports context-safe execution (`task_kind` where present).
7. **Test Coverage Mapping**: each task's acceptance criteria can be verified by an automated test; new test scaffolding must appear as an explicit dependency.
8. **Alternatives Considered**: when `decision.md` is required or present, it MUST contain at least 2 approaches with tradeoffs and a marked selection.

On standard/strict preset, any failed dimension is a blocker.
On light preset, dimension #1 (coverage) failures are advisory warnings; all other dimension failures remain blockers.
Record blocker and warning IDs by dimension.

## Sidecars
Apply the shared requirements checklist from `checklist-quality.md` while
auditing the bundle:
- specificity and measurability
- requirement-to-intent traceability
- edge cases and failure modes
- decision alternatives, trade-offs, and concrete risks

Audit tasks as execution units, not prose:
- split by bounded outcome, not file name alone
- require each task to name its evidence shape (`verdict` / `artifact` / `checklist`)
- keep rollout in reviewable batches; do not admit half-states that break the kernel between batches
- for guardrail-domain work, require the task to name a RED test plan before execution begins
- keep non-goals explicit, including at least one scope boundary and one rollout boundary

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
