# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/tmpl/templates/skills/spec-trace/SKILL.md:39` owns the report
    schema that explains how trace results are recorded.
  - `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl:11` owns the
    mandatory coverage matrix and currently has no explicit uncertain-result
    statuses.
  - `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl:43`
    delegates trace-edge enumeration to the attached spec-trace checklist.
  - `internal/tmpl/templates_test.go:1041` is the focused test seam for rendered
    review-template contract text.
- Dependency chains:
  - `internal/tmpl/templates/skills/spec-trace/*` -> rendered/exported
    `slipway-spec-trace` skill -> spec-compliance-review host and review/validate
    focus surfaces.
  - `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl` ->
    rendered governance host prompt -> governed Stage 1 review.
- Blast radius:
  - Low code blast radius if limited to authored templates and template tests.
  - Medium governance semantics risk because reviewer instructions affect how
    uncertain evidence is recorded.
- Constraints:
  - Generated surfaces are not the authoring surface.
  - Issue #157 explicitly asks for a skill-output-contract change and to avoid
    new engine prose heuristics.

### Patterns
- Existing conventions:
  - Spec-trace already uses lowercase per-row statuses:
    `covered`, `skipped`, and `drift` in
    `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl:12`.
  - Spec-compliance-review already fails closed when review evidence is missing
    or narrower than the requirement in
    `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl:50`.
  - Template contract regressions live in `internal/tmpl/templates_test.go`, with
    tests rendering the exact skill templates.
- Reusable abstractions:
  - Use existing markdown checklist/report-schema text; no new Go abstraction is
    necessary.
  - Use existing `Render` and `Content` helpers in `internal/tmpl` tests.
- Convention deviations:
  - None required. The change extends existing status vocabulary rather than
    replacing the report format.

### Risks
- Technical risks:
  - Medium: if `ambiguous` and `uncheckable` are described as benign skips, the
    original silent-pass problem remains.
  - Low: template wording changes can drift from generated output if tests do
    not assert the new contract.
  - Low: runtime compatibility risk is minimal because no engine-owned schema is
    changed.
- Guardrail domains:
  - `external_api_contracts`; unresolved or uncheckable review evidence must be
    auditable and fail closed for pass claims in sensitive domains.
- Reversibility:
  - High. The implementation is template/test only and can be reverted without
    data migration or runtime state migration.

### Test Strategy
- Existing coverage:
  - Template rendering tests cover review guidance wording but not Issue #157's
    uncertain per-item statuses.
  - Toolgen tests cover exported skill shape, not the semantic row statuses.
- Infrastructure needs:
  - No new fixtures or external tools.
  - Add a focused `internal/tmpl` regression test that fails against the current
    `covered | skipped | drift`-only contract.
- Verification approach:
  - RED: run the new focused template test before implementation and confirm it
    fails.
  - GREEN: update spec-trace and spec-compliance-review template text, then run
    the focused test.
  - BROAD: run relevant template/toolgen tests, full Go verification, and
    Slipway governance validation through `done_ready`.

### Options
- Option 1: extend the authored skill-output contract only.
  - Design: update spec-trace report schema/checklist to include
    `ambiguous`/`uncheckable` statuses with reason fields and coverage-gap
    accounting; update spec-compliance-review guidance to block unresolved
    ambiguity from pass claims; add template tests.
  - Tradeoffs: matches Issue #157's scoped remaining gap and keeps runtime blast
    radius low; relies on review prompt adherence rather than machine parsing.
- Option 2: add engine-owned parsing/validation of trace matrix rows.
  - Design: introduce a structured trace evidence parser and hard validation of
    row statuses.
  - Tradeoffs: stronger machine enforcement, but substantially broader and
    contrary to the issue's instruction to avoid new engine prose heuristics.
- Option 3: only update prose in spec-compliance-review.
  - Design: tell reviewers not to silently pass uncertain mappings, without
    changing spec-trace matrix vocabulary.
  - Tradeoffs: smallest edit, but leaves no first-class per-item status bucket,
    so it does not satisfy the remaining real scope.
- Selected: Option 1.
  - Rationale: it directly adds the missing recordable per-item uncertain
    statuses and coverage accounting while preserving existing covered/skipped/
    drift semantics and avoiding unnecessary runtime machinery.

## Unknowns
- Resolved: Is the old codebase map relevant? -> No. It was authored for issue
  #137 and has been re-authored inline for issue #157.
- Resolved: Is the implementation surface engine-owned or skill-output-owned?
  -> Skill-output-owned. The current gap is in spec-trace/checklist vocabulary
  and spec-compliance-review guidance.
- Remaining: None.

## Assumptions
- The final verification should stop at `done_ready`, not `slipway done` -
  Evidence: user objective says "直到done ready" and intake out-of-scope records
  finalization as excluded.
- Lowercase statuses should be used in matrix examples - Evidence:
  `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl:12` uses lowercase
  `covered`, `skipped`, and `drift`.
- Focused template tests are sufficient to prove the contract text changed -
  Evidence: existing tests in `internal/tmpl/templates_test.go:1041` assert
  rendered review-template contract wording.

## Canonical References
- `https://github.com/signalridge/slipway/issues/157`
- `internal/tmpl/templates/skills/spec-trace/SKILL.md:39`
- `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl:11`
- `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl:43`
- `internal/tmpl/templates_test.go:1041`
- `internal/toolgen/toolgen.go:254`
- `internal/engine/capability/registry_b2.go:72`
