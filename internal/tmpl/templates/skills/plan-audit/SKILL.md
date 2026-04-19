---
skill_id: plan-audit
name: slipway-plan-audit
description: "Use when validating that the governed artifact bundle is ready for execution. Triggers on post-authoring audit or whenever plan artifacts change materially."
---

# Plan Audit

```
IRON LAW: NO EXECUTION WITHOUT A VERIFIED, COMPLETE PLAN
```

Violating the letter of this rule is violating the spirit of this rule.

## Purpose
Validate that the governed artifact bundle is ready for execution. This is the
execution-readiness host skill that gates entry into governed implementation.
Mitigates: stale or incomplete plan bundle.

## Technique Overlay: `slipway-coding-discipline`
Use `slipway-coding-discipline` as the planning posture for this host:
- think before coding: surface hidden assumptions and scope ambiguity before execution starts
- simplicity first: prefer the smallest task graph that still covers the requirements
- surgical changes: keep target files precise and push unrelated cleanup out of the bundle
- goal-driven execution: keep detailed mechanics delegated to the owning governed skills

## Workflow Outline
1. Read the governed bundle and any durable codebase-map context.
2. Audit the artifact set, task dimensions, and requirement coverage.
3. Write verification, surface blockers or warnings, then wait for approval.

## When This Runs
Before wave execution begins. `slipway next` returns `next_skill: plan-audit`.

## Process

### 1. Read Context
Run `slipway next --json` and locate the governed change bundle.

If `input_context.codebase_map_dir` contains durable mapping documents, read at least:
- `ARCHITECTURE.md`
- `STRUCTURE.md`
- `TESTING.md`
- `CONCERNS.md`

Use them to verify task targets, blast radius assumptions, and test scaffolding needs. If the map is absent, continue, but call out the missing brownfield context as an advisory gap.

### 2. Validate Artifacts
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
- Stale propagation graph is clean (no invalidated downstream artifacts)

If `research.md` is present, also verify that:
- `## Alternatives Considered` is consistent with the selected approach in `decision.md`
- `## Canonical References` points to real docs/specs/code paths where possible
- `## Unknowns` and `## Assumptions` have been addressed or are consistent with the plan

Any missing artifact from the required set is a **blocker**.

### 2z. Plan Size Advisory
A single wave SHOULD contain no more than 5 tasks. If a wave exceeds this threshold, consider splitting into smaller waves or decomposing large tasks. This is advisory — the auditor SHOULD flag oversized waves as a warning (not a blocker) and recommend splitting when task count suggests context rot risk.

### 2a. Dimension Checklist (8D)
The plan audit MUST explicitly check all eight dimensions:
1. **Coverage (Nyquist Check)**: every requirement in requirements.md SHOULD have at least one corresponding task in tasks.md. Walk through each requirement block in `requirements.md`, record its stable `REQ-*` ID, and verify at least one task lists that same `REQ-*` ID in its `covers` field. On **standard/strict** preset, uncovered requirements are blockers. On **light** preset, uncovered requirements are advisory warnings.
2. **Completeness**: each task has objective, explicit `wave`, dependencies, and expected outputs.
3. **Dependency Integrity**: `depends_on` references valid tasks, contains no cycles, and only points to earlier waves.
4. **Key Links**: each task links to concrete `target_files` or evidence targets.
5. **Scope Control**: task targets stay within declared change scope boundaries.
6. **Context Compliance**: task metadata supports context-safe execution (task_kind where present).
7. **Test Coverage Mapping**: each task's acceptance criteria can be verified by an automated test. Identify any test scaffolding tasks (Wave 0) needed before execution begins. Tasks that require new test infrastructure MUST declare it as a dependency.
8. **Alternatives Considered**: when `decision.md` is required or present, it MUST contain a substantive `## Alternatives Considered` section listing at least 2 approaches with tradeoff analysis and a marked selection. A missing or empty alternatives section is a blocker — but only for paths that actually use `decision.md`.

On standard/strict preset, any failed dimension is a blocker. On light preset, dimension #1 (coverage) failures are advisory warnings; all other dimension failures remain blockers. Record blocker and warning IDs by dimension.

### 2c. Requirements Quality Sidecar
Apply the shared requirements checklist from `checklist-quality.md` while auditing the bundle:
- specificity and measurability
- requirement-to-intent traceability
- edge cases and failure modes
- decision alternatives, trade-offs, and concrete risks

### 3. Write Verification
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

### 4. Present and Advance
Show audit results. <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway next` until the user approves.</HARD-GATE>

After confirmation: `slipway next`

## DO NOT SKIP
1. Confirm verification file exists after audit.
2. Stop on audit blockers.
3. Re-run when artifacts change.

See `references/audit-smells.md` for recurring audit rationalizations and
blocker patterns. Keep the inline host focused on artifact checks, not the full
smell catalog.

## Hard Gate Enforcement
DO NOT advance past plan audit until the user has explicitly confirmed the audit results. Presenting findings is not the same as receiving approval. Wait for the user to approve before calling `slipway next`.

**Anti-pattern**: "All artifacts look valid, advancing automatically." — Plan audit exists to catch stale or incomplete bundles. Even when the audit passes, the user must confirm because they may have domain knowledge the audit cannot detect.

## Step Declaration
Declare current step and expected output before executing each workflow step.
