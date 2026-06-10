# Structure

- `internal/tmpl/templates/skills/spec-trace/`
  - `SKILL.md`: frontmatter, purpose, report schema, anti-patterns.
  - `CHECKLIST.tmpl`: attached checklist and coverage matrix.
- `internal/tmpl/templates/skills/spec-compliance-review/`
  - `SKILL.md.tmpl`: Stage 1 review host instructions that embed spec-trace
    guidance.
  - `references/checklist-quality.md`: adjacent reference read by the host.
- `internal/tmpl/templates_test.go`
  - Existing focused regression area for template contract wording.
- `internal/toolgen/`
  - Exports governance and technique skill surfaces; expected to remain
    structurally unchanged for this issue.
- `artifacts/changes/resolve-github-issue-157-add-uncheckable-inconclusive-per-it/`
  - Governed artifact bundle and verification evidence for this change.
