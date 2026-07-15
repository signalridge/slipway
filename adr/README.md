# Architecture Decision Records

This directory contains maintainer-facing records of significant technical choices. ADRs explain context, alternatives, and consequences; they are not user documentation, release instructions, or a second runtime specification.

Accepted records are kept as historical rationale. When a decision changes, add a new ADR that supersedes the earlier record instead of rewriting its original decision.

| ID | Decision | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-source-bundle-v2.md) | Use manifest-addressed source bundles | Accepted | 2026-07-13 |

Current behavior must still be verified against implementation, tests, and the [versioned machine schema](../docs/reference/v2/machine-protocol.schema.json). User-facing behavior belongs under [`docs/`](../docs/en/index.md), and executable evidence belongs under [`acceptance/`](../acceptance/README.md).
