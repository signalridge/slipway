# Integrations

- External APIs: none expected for this change.
- Infrastructure bindings: generated host adapter surfaces are local files
  produced by `internal/toolgen`; no remote service integration is required.
  Evidence: `internal/toolgen/toolgen_test.go:631-666`.
- Datastores and queues: none expected.
- File formats and protocols: markdown templates under `internal/tmpl/templates`;
  generated skill frontmatter; governed artifacts under
  `artifacts/changes/<slug>/`; runtime handoff path
  `.git/slipway/runtime/changes/<slug>/handoff.md`. Evidence:
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl:1-6`,
  `cmd/session_start_hook.go:126-135`.
- Notes: the runtime handoff file is local runtime context, not a tracked
  governed artifact or external protocol.
