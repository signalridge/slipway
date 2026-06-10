# Testing

- Existing coverage:
  - `internal/tmpl/templates_test.go:1041` verifies review-template guidance for
    negative-path evidence.
  - `internal/tmpl/templates_test.go:1063` verifies spec-compliance-review and
    spec-trace contract wording around literal clause coverage.
  - `internal/tmpl/templates_test.go:1082` verifies pending-decision guidance.
  - `internal/toolgen/toolgen_test.go:1718` verifies generated skill typed parts
    include the expected spec-trace checklist section.
- Gap for Issue #157:
  - No focused regression currently asserts `ambiguous` or `uncheckable` item
    statuses, required reasons, or coverage-gap accounting in spec-trace.
  - No focused regression currently asserts spec-compliance-review blocks pass
    claims when unresolved ambiguity remains.
- Planned verification:
  - Add a targeted failing test in `internal/tmpl/templates_test.go` before
    template edits.
  - Run the targeted test after edits.
  - Run broader relevant tests (`go test ./internal/tmpl ./internal/toolgen`),
    then full repository verification before closeout.
