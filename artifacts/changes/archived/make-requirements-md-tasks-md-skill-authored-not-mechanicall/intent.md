# Intent

## Summary
Make requirements.md/tasks.md skill-authored, not mechanically seeded with fake content that passes structure-only validation (issue #91): the engine yields an obviously-not-real scaffold instead of fabricated tautology requirements/tasks, and governance validates substance (MUST/SHALL in the requirement body, a concrete non-tautological scenario, placeholder rejection) so a mechanical scaffold cannot reach done. Engine owns structure; skill owns substance.
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->
Spans engine artifact authoring (seed functions + templates), two governance validators (requirements + tasks substance), a runtime placeholder gate, and a new public CLI surface (`slipway instructions`) with generated-surface regeneration. It changes external governance contracts and must fail closed, so it is complex rather than simple.

## Guardrail Domains
<!-- none detected -->
None. This is governance/authoring correctness, not auth/credentials/PII/financial/schema-migration/irreversible work. (Public CLI/JSON behavior still reviewed as an external contract.)

## In Scope
Broader scope (A + C + tasks gate + runtime application + instructions surface), confirmed with the user:
- **A — engine yields structure, not fabricated substance** (`internal/engine/artifact/manager.go`, `internal/tmpl/templates/artifacts/requirements.md`, `internal/tmpl/templates/artifacts/tasks.md`): the default `seedRequirements`/`appendRequirementBlock` and `seedTasks` output becomes an *obviously-not-real* honest placeholder scaffold (headings + HTML-comment guidance + explicit "replace with…" prose that embeds the quality bar), matching the honesty of `seedDecision`/`seedResearch`. Fabricated tautology requirement sentences and the tautology `GIVEN/WHEN/THEN` are removed. The `--from-doc` path may still derive requirement/task *titles* from the user-provided doc, but must not fabricate normative bodies or tautology scenarios.
- **C — substance gate (requirements)**: add a requirements-specific `LooksLikeRequirementsPlaceholder` (`requirements.go`) matching only the requirements seed markers + anchored tautology lines (not the generic decision/research sentinels, so authored prose containing a generic phrase is not false-flagged); extend `EvaluateRequirementsContract` (`internal/engine/artifact/requirements_contract.go`) so each `REQ-*` body line contains an RFC-2119 strong-obligation keyword (`MUST`/`SHALL`/`REQUIRED`), each requirement has ≥1 concrete (non-tautology, non-placeholder) `#### Scenario` complete within a single scenario, and placeholder content is rejected.
- **C — substance gate (tasks)**: add a tasks substance validator that rejects placeholder tasks (`pending task objective` / `pending verification objective`) and requires real objectives; wire it into the same gate path `validate`/plan-audit uses for `requirements_contract`.
- **runtime scoping** (`internal/engine/governance/runtime_actions.go`): keep the `artifactSectionHasSubstantiveContent` placeholder helper scoped to `decision.md` (its only invoked artifacts are `decision.md`/`assurance.md`). Placeholder `requirements.md`/`tasks.md` rejection is owned by the progression substance gate + the validate contracts, so generalizing this runtime helper would be dead, unreachable code.
- **instructions surface**: add `slipway instructions <artifact>` (openspec-style) returning the artifact template + authoring guidance (text + `--json`), so an authoring skill can read the template and quality bar before writing. Register the command, its capability/command surface, and regenerate generated command references/skills/docs with zero toolgen drift.
- Tests for every behavior; `go build ./... && go vet ./... && go test ./...` green; toolgen self-loop zero drift.

## Out of Scope
- Routing authorship to skills is already the de-facto behavior (skills author requirements/tasks; with this change the engine stops fabricating). No new skill→artifact routing wiring beyond the honest scaffold + instructions surface.
- The markdown format itself stays (`### Requirement:` / `#### Scenario:` / `- [ ] \`t-NN\``); we change content honesty + validation, not the schema shape.
- Issue #92 (separate, already done-ready).

## Constraints
- Fail closed: the substance gate must reject mechanical/placeholder content; it must not introduce a bypass/force path. Unknown/edge inputs default to rejection only when clearly placeholder, not on legitimately-authored content (avoid false positives that block real work).
- The new gate must not retroactively break legitimately-authored bundles (real MUST/SHALL + concrete scenarios pass).
- Keep generated surfaces aligned (zero drift) — code, skills, command refs, docs are one product surface.

## Acceptance Signals
- A mechanical/placeholder `requirements.md` (engine default scaffold, unedited) makes `EvaluateRequirementsContract` return `invalid` and cannot reach `done`; a real authored one returns `valid`.
- `LooksLikeRequirementsPlaceholder` returns true for the requirements seed/tautology sentinels but NOT for generic decision/research sentinels in authored prose; placeholder `requirements.md`/`tasks.md` is rejected by the progression substance gate + the validate contracts; the runtime helper stays scoped to `decision.md`.
- A placeholder `tasks.md` is rejected by the tasks substance validator; a real one passes.
- The engine's default `seedRequirements`/`seedTasks` output is detected as placeholder (obviously-not-real), proving the engine no longer fabricates plausible substance.
- `slipway instructions requirements` / `slipway instructions tasks` return the template + authoring guidance (text and `--json`).
- Toolgen self-loop reports zero drift; `go build ./... && go vet ./... && go test ./...` pass.

## Open Questions
- [x] None — needs_discovery=false; root cause and fix surfaces already confirmed by reading current code (manager.go seed funcs + templates, requirements_contract.go, runtime_actions.go).

## Deferred Ideas
- Quantitative ambiguity scoring (gsd-style) for requirements — out of scope; a future enhancement.

## Approved Summary
Engine owns structure, skill owns substance. (A) The engine's default requirements.md/tasks.md scaffold becomes obviously-not-real honest placeholders instead of fabricated tautology requirements/tasks. (C) A substance gate rejects mechanical/placeholder content: a requirements-specific placeholder matcher, a `EvaluateRequirementsContract` content check (RFC-2119 MUST/SHALL/REQUIRED-in-body + concrete single-scenario + placeholder rejection), and a tasks substance validator; the runtime placeholder helper stays scoped to decision.md (requirements/tasks substance is owned by the progression gate + validate contracts). A new `slipway instructions <artifact>` surface serves the template + guidance to authoring skills. All generated surfaces regenerated with zero drift; full build/vet/test green. Net: a 100% mechanical scaffold cannot pass validation or reach done.

Confirmed by user 2026-06-06 (selected the broader "A+C + tasks gate + runtime application + instructions surface" scope).
