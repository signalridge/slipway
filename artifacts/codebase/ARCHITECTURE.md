# Architecture

Re-authored for change `resolve-github-issue-157-add-uncheckable-inconclusive-per-it`
(GitHub issue #157).

Question: where should per-item ambiguous/uncheckable spec verification status
be expressed so "could not check" is auditable and never silently passes?

- Affected modules:
  - `internal/tmpl/templates/skills/spec-trace/SKILL.md:39` defines the
    spec-trace report schema consumed by review users.
  - `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl:11` defines the
    mandatory coverage matrix and currently limits item status to
    `covered`, `skipped`, and `drift`.
  - `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl:43`
    tells reviewers to use the attached spec-trace checklist while performing
    Stage 1 governed review.
  - `internal/tmpl/templates_test.go:1041` already pins review-template contract
    wording, making it the focused regression-test seam for this change.
  - `internal/toolgen/toolgen.go:254` and `internal/toolgen/toolgen.go:369`
    export governance/technique skills; spec-trace is an exported technique
    helper rather than a lifecycle-owned host.
- Dependency flow:
  - Authored templates under `internal/tmpl/templates/skills/...` are the source
    of truth.
  - Template rendering tests read authored content directly or render `.tmpl`
    files and assert required contract text.
  - Tool generation exports the authored skill surfaces; this change should not
    edit generated copies directly.
- Coupling hotspots:
  - `spec-compliance-review` depends on spec-trace for the trace matrix contract,
    so changing spec-trace wording is the main architectural fix.
  - Adding runtime parsing or schema validation would widen the blast radius into
    engine capability and verification-state code; Issue #157 explicitly steers
    away from engine prose heuristics.
- Blast radius:
  - Low implementation blast radius: authored skill templates and focused tests.
  - Higher workflow semantics sensitivity: the wording controls how review
    agents classify uncertain mappings in governed review.
