# Integrations

- No external network, database, or service integration is expected.
- User-facing integration surface:
  - Generated/exported skill prompts for `spec-trace` and
    `spec-compliance-review`.
  - Governed review flows that embed spec-trace during spec-compliance review.
- Internal integration surface:
  - Template rendering (`internal/tmpl`) and tool generation (`internal/toolgen`)
    consume authored skill files.
  - Capability registry bindings in `internal/engine/capability/registry_b2.go:72`
    identify spec-trace as a host-embedded checklist for spec-compliance review.
