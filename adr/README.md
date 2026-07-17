# Architecture Decision Records

This directory contains maintainer-facing records of significant technical choices. ADRs explain context, alternatives, and consequences; they are not user documentation, release instructions, or a second runtime specification.

Accepted records are kept as historical rationale. When a decision changes, add a new ADR that supersedes the earlier record instead of rewriting its original decision.

| ID | Decision | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-source-bundle-v2.md) | Use manifest-addressed source bundles | Accepted | 2026-07-13 |

The complete Chinese contract in [issue #434](https://github.com/signalridge/slipway/issues/434) and the [versioned machine schema](../docs/reference/v2/machine-protocol.schema.json) define the intended contract. Implementation exposes current behavior, while tests and [`tests/acceptance/`](../tests/acceptance/README.md) record observations rather than readiness or release authority. User-facing behavior belongs under [`docs/`](../docs/en/index.md).
