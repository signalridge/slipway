# Structure

Re-authored for change
`resolve-github-issue-151-thin-host-disk-handoff-return-contr`
(GitHub issue #151).

- `internal/tmpl/templates/skills/research-orchestration/SKILL.md`
  - Static governed host for discovery/research artifacts.
- `internal/tmpl/templates/skills/plan-audit/SKILL.md`
  - Static governed host for S1 plan readiness and plan-audit evidence.
- `internal/tmpl/templates/skills/intake-clarification/SKILL.md`
  - Static governed host for S0 intent clarification evidence.
- `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`
  - Templated governed host for S3 spec compliance review.
- `internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl`
  - Templated governed host for S3 code quality review.
- `internal/tmpl/thin_host_content_test.go`
  - Focused thin-host regression tests for issue #114-style bounded host
    contracts.
- `internal/tmpl/templates_test.go`
  - Broader template rendering and generated-contract regression tests.
- `internal/toolgen/toolgen_test.go`
  - Exported skill generation coverage; useful if issue #151 changes must be
    proven after tool generation.
- `artifacts/changes/resolve-github-issue-151-thin-host-disk-handoff-return-contr/`
  - Governed artifact bundle and verification evidence for this change.
