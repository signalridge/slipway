# Testing

Re-authored for change `fix-pending-approach-reported-as-locked-decision`
(issue #140); replaces the stale issue-#114 map.

- Test layout: Go `*_test.go` files alongside packages.
- Coverage hotspots for this change:
  - `cmd/next_skill_constraints_test.go` — `TestParseDecisionItems`,
    `TestBuildSkillConstraintsLockedVsPending`, `TestPlanLockedFromGates`, and
    `TestSkillConstraintsPendingDecisionsBeforePlanLock` /
    `TestSkillConstraintsLockedDecisionsAfterPlanLock` (locked-vs-pending split
    assembly plus full `next --json` pending and confirmed paths).
  - `internal/engine/artifact/*_test.go` — decision parsing / placeholder
    detection.
  - `internal/tmpl/templates_test.go` / `internal/toolgen/toolgen_test.go` —
    generated `spec-compliance-review` surface text contracts (assert the new
    pending advisory is present after regeneration).
- Coverage gaps to close:
  - No known blocking gap for issue #140. The unconfirmed path is covered by
    `TestSkillConstraintsPendingDecisionsBeforePlanLock`; the confirmed path is
    covered by `TestSkillConstraintsLockedDecisionsAfterPlanLock`, which drives
    real `next --json --diagnostics` output with G_plan approved.
- Verification commands: `go build ./...`; `go vet ./...`; `go test ./...`.
- Fixture patterns:
  - cmd tests build a governed change/view in temp dirs and assert on the
    rendered JSON struct.
  - Template/toolgen tests read the embedded template FS and assert on rendered
    markdown substrings.
- Notes / source references:
  - `cmd/next_skill_constraints_test.go`, `internal/engine/artifact`,
    `internal/tmpl/templates_test.go`, `internal/toolgen/toolgen_test.go`.
