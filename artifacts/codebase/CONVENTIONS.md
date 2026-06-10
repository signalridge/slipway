# Conventions

- Source of truth:
  - Edit authored skill templates under `internal/tmpl/templates/skills/...`.
  - Do not edit exported/generated skill copies directly.
- Wording:
  - Gate-bearing instructions should use explicit, parseable vocabulary.
  - Keep new terms lowercase in markdown examples (`ambiguous`, `uncheckable`)
    to match existing `covered`, `skipped`, and `drift` status style.
- Testing:
  - Use package-local Go template tests in `internal/tmpl/templates_test.go`.
  - Assert behaviorally meaningful phrases, not incidental formatting.
- Scope control:
  - Prefer contract text and tests over new runtime/schema machinery when the
    issue is about AI-facing review instructions.
