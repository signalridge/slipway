# Domain: ops-and-diagnostics

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `incident-response` | T3 | commands `status`, `health`; export-only |

Role:

1. Classify severity, reconstruct timeline, and drive PIR flow.
2. Never enters `repair` — T3 stays read-only on governed code paths.

Notes:

- Absorbs `incident-commander`, `incident-response`, and
  `acceptance-orchestrator` gate posture.
- Consumes view-only surfaces (`observability-query`, `sentry`) as evidence,
  but does not wrap them.
