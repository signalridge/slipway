# Live GitHub fixture instructions

No live fixture was executed or modified for this change. These instructions define how to collect G evidence later without confusing it with the local fault harness.

## Isolation and credentials

- Use a dedicated protected GitHub test account and a disposable **user-owned** repository with Issues enabled. Do not depend on Organization Issue Types, Issue Fields, or Projects.
- Store the least-privilege token only in protected CI/environment secret storage. Never put it in argv logs, URLs, Issues, fixtures, transcripts, journals, or repository files.
- Never expose the secret to fork pull requests. Live jobs must be disabled for untrusted contributors and require an approved protected environment.
- Enable private vulnerability reporting only when testing an actual vulnerability workflow. A normal public Issue has no private switch; do not use real sensitive text.
- Record GitHub/`gh` versions, fixture repository node ID, initial fixture commit, and timestamps. Sanitize account/repository identifiers in public evidence if policy requires it.

## Fixture coverage

1. Create exact `level:objective`/one `kind:*` and `level:change`/one `kind:*` Issues with approved operation/item UUID markers and body files.
2. Verify native parent/sub-issue and blocked-by relationships in a user-owned repository without a Project.
3. Exercise `gh >= 2.94.0` first-class operations and a controlled older-`gh` or missing-command environment using the official REST fallback.
4. Exercise permission-limited behavior and verify `environment_unavailable` rather than local substitute authority.
5. Read back body digest, markers, labels, repository/Issue node IDs, parent, dependencies, canonical URL, and old URL alias after a same-`github.com` transfer. Do not trust a cross-host redirect.
6. Approach the documented 100 sub-issue and 50-per-direction dependency boundaries only in a disposable fixture and clean up manually after evidence review.
7. Use a controlled proxy/fault layer for timeout-after-success, partial relation failure, duplicate marker creation, and indexing delay; reconcile through paginated non-search APIs and record `created|matched|failed|ambiguous` without blind create retry.

## Evidence and cleanup

Store sanitized request/response metadata, marker UUIDs, body digests, observed URLs, relation readback, statuses, and evaluator notes outside the product journal. Do not store tokens, raw private comments, complete transcripts, environment dumps, or hidden reasoning. Record partial successes before manual cleanup; never claim rollback or exactly-once behavior.

After collection, manually close/delete disposable resources according to the test-account retention policy and revoke/rotate credentials. Cleanup is not part of publication reconciliation and must not rewrite evidence.

`../github_publication_fault_harness.py` is deterministic, network-free H/G-adjacent evidence. It is useful in ordinary CI but **cannot substitute for this live GitHub.com G fixture**.
