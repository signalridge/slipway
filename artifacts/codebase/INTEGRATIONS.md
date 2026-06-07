# Integrations

- External APIs:
  - GitHub issue #114 is the external problem statement and acceptance context.
- Infrastructure bindings:
  - Generated skill surfaces target multiple agent runtimes; Codex, Claude,
    Cursor, Gemini, and other supported tools may expose different subagent
    mechanics.
- Datastores and queues:
  - Not applicable.
- File formats and protocols:
  - Governance verification artifacts are YAML files under
    `artifacts/changes/<slug>/verification/`.
  - Runtime task evidence is recorded through `slipway evidence task` and
    referenced from wave verification.
- Notes:
  - Runtime portability is part of the design constraint: instructions should
    describe delegated verifier/executor context without requiring a
    Claude-only API name as the only satisfy path.
