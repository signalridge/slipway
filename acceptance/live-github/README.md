# Live GitHub fixture collection

These instructions describe how to collect live GitHub evidence without confusing it with the deterministic local publication-fault harness. Current collection status belongs in the [acceptance matrix](../README.md), not in this procedure.

## Isolation and credentials

- Use a dedicated protected GitHub test identity and disposable repositories with Issues enabled.
- Collect both personal-account-owned and organization-owned repository coverage when the behavior under test concerns ownership. If only one ownership type is collected, state that limitation explicitly.
- Do not depend on GitHub Projects, Organization Issue Types, or Organization Issue Fields for the baseline fixture.
- Store the least-privilege token only in protected CI or environment secret storage. Never put it in argv logs, URLs, Issues, fixtures, transcripts, journals, or repository files.
- Never expose credentials to fork pull requests. Live jobs must be disabled for untrusted contributors and require an approved protected environment.
- Use only synthetic, non-sensitive text. A normal public Issue has no private switch.
- Record GitHub and `gh` versions, repository and Issue node IDs, fixture revision, ownership type, permissions, and timestamps. Sanitize account or repository names in public evidence when policy requires it.

## Fixture coverage

1. Create exact Objective and Change markers with one managed `kind:*` label and approved operation/item identifiers.
2. Verify native parent/sub-issue and blocked-by relationships without requiring a Project.
3. Repeat ownership-sensitive source and relationship checks in both personal-account-owned and organization-owned repositories.
4. Exercise `gh >= 2.94.0` first-class operations and a controlled older or missing `gh` environment using the official REST fallback.
5. Exercise permission-limited behavior and verify `environment_unavailable` instead of creating a local substitute authority.
6. Read back body digests, markers, labels, repository/Issue node IDs, parent, dependencies, canonical URL, and old URL alias after a same-`github.com` transfer. Do not trust a cross-host redirect.
7. Approach the documented sub-issue and dependency limits only in disposable fixtures; record the observed API behavior and clean up manually.
8. Use a controlled proxy or fault layer for timeout-after-success, partial relation failure, duplicate markers, and indexing delay. Reconcile through paginated non-search APIs and record `created|matched|failed|ambiguous` without blind create retry.

## Evidence and cleanup

Store sanitized request/response metadata, marker identifiers, body digests, observed URLs, relation readback, statuses, ownership type, and evaluator notes outside the product journal. Do not store tokens, raw private comments, complete transcripts, environment dumps, or hidden reasoning.

Record partial successes before manual cleanup; never claim rollback or exactly-once behavior. After collection, close or delete disposable resources according to the test-account retention policy and revoke or rotate credentials. Cleanup must not rewrite evidence.

`../github_publication_fault_harness.py` is deterministic, network-free adjacent evidence. It is useful in ordinary CI but cannot substitute for a live GitHub fixture.
