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

## Disk-Handoff Contract
For bulky audit artifacts, keep the coordinator thin:
- Pass required-reading paths from `slipway next --json`; do not paste artifact
  bodies or codebase-map documents into the host context.
- An isolated subagent writes bulky artifacts directly to disk under
  `artifacts/changes/<slug>/` and returns only a short confirmation naming the
  paths written and any blockers.
- The confirmation is a claim, not evidence. The host must inspect the written
  files and record the verdict through `slipway evidence skill`; do not
  hand-edit verification YAML into a pass state.
- Slipway CLI owns `run_version`, timestamps, freshness inputs, and verdict
  stamping; subagents must not self-stamp freshness or final verdicts.

## Author Substance First
The engine owns each plan artifact's structure; you own its substance. Author the
real plan artifacts in schema dependency order: `requirements.md` first,
`decision.md` next on the expanded schema, then `tasks.md`. Run `slipway validate`
and read its `requirements_contract` / `decision_contract` / `tasks_contract`,
and watch for `missing_required_artifact:decision.md`: an `invalid` contract or
a missing required artifact means the substance was never authored.

When that happens, author the real content first — do not audit a placeholder.
Each `slipway instructions <artifact>` payload carries the template, the quality
bar, the resolved output path to write, and the upstream dependencies to read by
path; honor its `context`/`rules` but never copy them into the file:
- `slipway instructions requirements` — each `REQ-*` body states what the system
  MUST, SHALL, or is REQUIRED to do (an RFC-2119 strong-obligation keyword), with
  at least one concrete `#### Scenario` (real GIVEN/WHEN/THEN, no tautology).
- `slipway instructions decision` — each of the five sections (Alternatives
  Considered, Selected Approach, Interfaces and Data Flow, Rollout and Rollback,
  Risk) carries concrete, change-specific content. On a discovery change, use the
  selected approach and evidence recorded in `research.md`; author the formal
  decision here after the requirements are real.
- `slipway instructions tasks` — each task carries a concrete objective plus
  `depends_on`, `target_files`, `task_kind`, and `covers`. The engine assigns
  waves from `depends_on` and `target_files`; do not author a `wave:` line — a
  hand-declared wave is rejected fail-closed. Every task names concrete
  `target_files` that bound the files or evidence targets it changes or
  verifies.

Author task width for the computed schedule. Keep `depends_on` honest: it is a
scheduling input, not narrative order, and a dependency that is not a real
execution-order constraint needlessly serializes the waves. Keep `target_files`
precise: name exact files rather than directories or globs, because overlapping
or broad targets bump tasks into later waves and flatten the schedule. Absorb
small same-file steps into one task instead of splitting work that can never
run concurrently.

A mechanical or vacuous plan artifact cannot reach done. Re-run `slipway validate`
until the substance gate passes, then audit.

## Validate Artifacts
Verify the **required artifact set** exists and is structurally valid:
- Required (all paths): `change.yaml`, `intent.md`, `requirements.md`, `tasks.md`.
  `decision.md` is intentionally listed separately below because only expanded
  schemas require it.
- Required whenever the change is on the **expanded** artifact schema — discovery
  changes (`needs_discovery=true`); the research/config/meta workflow profiles; or
  a repo configured to default to expanded. A standard non-discovery code change
  uses the **core** schema and does not require it. No public surface exposes the
  frozen schema name, so treat the engine as the authority: a missing one on an
  expanded change is surfaced as `missing_required_artifact:decision.md`.
- Required when `needs_discovery=true` (the only discovery-gated artifact — check
  `needs_discovery` in `slipway next --json` / `slipway validate`): `research.md`.
  A missing one is a blocker (`missing_required_artifact:research.md`). On
  non-discovery changes `research.md` is absent and not required.
- `assurance.md` is NOT audited at S1 plan-audit. It is a review/verify-phase
  deliverable deferred to `S3_REVIEW` authoring (it does not exist yet at plan
  time and must not be authored now). Its existence and structure are enforced
  solely at `S3_REVIEW` and later by the assurance contract gate.

Each required plan artifact must be non-empty, structurally valid, free of stale
code references, and consistent with the stale-propagation graph. Tasks must have
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
2. **Completeness**: each task has objective, dependencies, and expected outputs.
3. **Dependency Integrity**: every `depends_on` reference resolves and has no cycles, and every dependency is a real execution-order dependency — reject fabricated or narrative-only dependencies that needlessly serialize the computed waves.
4. **Key Links**: each task names concrete `target_files` for the files or evidence targets it changes or verifies.
5. **Scope Control**: task targets stay inside declared scope.
6. **Context Compliance**: task metadata supports context-safe execution (`task_kind` where present).
7. **Test Coverage Mapping**: acceptance criteria map to automated checks; new scaffolding is an explicit dependency.
8. **Alternatives Considered**: required/present `decision.md` names at least 2 approaches, tradeoffs, and the selected approach.

On standard/strict, any failed dimension blocks. On light, only dimension #1
coverage failures downgrade to warnings.

## Sidecars
Apply `references/checklist-quality.md` (shipped next to this skill):
specificity, measurability, requirement-to-intent traceability, edge cases,
failure modes, alternatives, tradeoffs, and concrete risks.

Audit tasks as execution units, not prose:
- split by bounded outcome, not file name alone
- require each task to name its evidence shape (`verdict` / `artifact` / `checklist`)
- require task acceptance criteria to be satisfiable during S2 execution; do
  not accept criteria that require future S3 review or S4 closeout evidence
  before the workflow can legally reach those states
- keep rollout in reviewable batches; do not admit half-states that break the kernel between batches
- for guardrail-domain work, require a RED test plan before execution
- keep non-goals explicit, including scope and rollout boundaries

## Author/Auditor Dispatch And Context-Origin Handles
Dispatch the audit work in a host-native subagent that runs on the SHARED change
worktree (the same worktree that holds the governed bundle; there is no
per-audit git worktree isolation). Model the dispatch on how
`slipway-wave-orchestration` fans out executor subagents: spawn one fresh-context
subagent with a bounded audit brief, pass it required-reading paths rather than
artifact bodies, wait for its result, and record its handle. If a capable runtime
cannot spawn, wait for, and close a subagent, stop and ask for operator direction
rather than auditing inline in the host context.

Plan-audit records an author/auditor PAIR of context handles, NOT a
`context_origin:stage=` token. Record both:
- `plan_origin:<handle>` — the handle/identity of the context that AUTHORED the
  plan bundle (`requirements.md`, `decision.md`, `tasks.md`).
- `audit_origin:<handle>` — the handle/identity of the dispatched subagent that
  AUDITED the bundle in fresh context.

The engine consumes the two as a pair and compares them for independence; a
missing handle, or one uniform handle stamped as both author and auditor, is the
degenerate single-context signature the gate rejects.

## Record Verification
Write bulky audit notes to disk, then record the verdict through the CLI so
Slipway owns the timestamp, `run_version`, freshness inputs, and digest stamp.
Do not hand-edit `verification/plan-audit.yaml`.

```bash
slipway evidence skill \
  --skill plan-audit \
  --verdict pass \
  --reference "plan-audit:pass" \
  --reference "plan_origin:<handle>" \
  --reference "audit_origin:<handle>" \
  --notes-file artifacts/changes/{slug}/verification/plan-audit-notes.md
```

## Present and Advance
Show audit results. <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway run` (the advancing command) until the user approves; `slipway next` is read-only preview and never advances.</HARD-GATE>

After confirmation, advance with `slipway run`.

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
