# Architecture Decision Records

This directory contains maintainer-facing records of significant technical choices. ADRs explain context, alternatives, and consequences; they are not user documentation, release instructions, or a second runtime specification.

Accepted records are kept as historical rationale. When a decision changes, add a new ADR that supersedes the earlier record instead of rewriting its original decision.

| ID | Decision | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-source-bundle-v2.md) | Use manifest-addressed source bundles | Accepted | 2026-07-13 |
| [0002](0002-seventh-capability-workflow.md) | Add a seventh guidance-only capability (`slipway-workflow`) | Accepted | 2026-07-18 |
| [0003](0003-scope-workflow-to-slipway-functions.md) | Scope `slipway-workflow` to Slipway functions, not skill catalogs | Accepted | 2026-07-18 |

The base Chinese contract in [issue #434](https://github.com/signalridge/slipway/issues/434), later accepted ADRs, and the [versioned machine schema](../docs/reference/v2/machine-protocol.schema.json) together define the intended contract. [ADR-0002](0002-seventh-capability-workflow.md) adds the seventh capability and reaffirms the no-router boundary; [ADR-0003](0003-scope-workflow-to-slipway-functions.md) scopes it to lifecycle routing across Slipway's own functions. Neither changes the machine protocol or Run semantics. Implementation exposes current behavior, while tests and [`acceptance/`](../acceptance/README.md) record observations rather than readiness or release authority. User-facing behavior belongs under [`docs/`](../docs/en/index.md).
