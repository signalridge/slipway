# Architecture Decision Records

This directory contains maintainer-facing records of significant technical choices. ADRs explain context, alternatives, and consequences; they are historical decision records, not user documentation, release instructions, or a second runtime specification.

Accepted records are kept as historical rationale. When a significant architectural decision changes, add a new ADR that supersedes the earlier record instead of rewriting its original decision. Routine bug fixes, implementation corrections, and documentation synchronization do not require an ADR.

| ID | Decision | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-source-bundle-v2.md) | Use manifest-addressed source bundles | Accepted | 2026-07-13 |
| [0002](0002-seventh-capability-workflow.md) | Add a seventh guidance-only capability (`slipway-workflow`) | Accepted | 2026-07-18 |
| [0003](0003-scope-workflow-to-slipway-functions.md) | Scope `slipway-workflow` to Slipway functions, not skill catalogs | Accepted | 2026-07-18 |

The Chinese proposal in [issue #434](https://github.com/signalridge/slipway/issues/434) records the original product goals, design intent, and rationale. It remains useful context, but it is mutable and its details may be superseded by later decisions or implementation evolution. Accepted ADRs record significant decisions made along that path. The versioned [machine protocol](../docs/reference/v2/machine-protocol.schema.json) and [source-envelope](../docs/reference/v2/source-envelope.schema.json) schemas are authoritative only for the serialization shapes they cover. [ADR-0002](0002-seventh-capability-workflow.md) adds the seventh capability and reaffirms the no-router boundary; [ADR-0003](0003-scope-workflow-to-slipway-functions.md) scopes it to lifecycle routing across Slipway's own functions. Neither changes the machine protocol or Run semantics.

For a given revision, its code, built `--help`, generated capabilities, user documentation, and observable behavior describe the implemented product and must be kept coherent. A tagged release, its release notes, and its actual artifacts state what users can obtain as published behavior; repository intent or a green build does not establish that an artifact was released. Tests, [`acceptance/`](../acceptance/README.md), and CI are evidence from particular revisions and executions. They do not set product direction or hold merge, readiness, or release authority.
