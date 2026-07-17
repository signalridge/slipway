# Acceptance fixtures

`container-source-envelope.json` is a deterministic, synthetic, credential-free valid Change source used by the post-release container smoke.

`github-publication-faults.json` is deterministic, credential-free input for `../github_publication_fault_harness.py`. UUIDs, repository names, URLs, Issue numbers, and text are fictional.

The fixture covers:

- a new Change draft containing only operation/item receipt markers, with no `change/v2` marker or manifest;
- section comments beginning with their section marker and containing no `slipway-level` marker;
- a final Change beginning with the level marker and manifest fence, with the same receipt markers after the fence;
- exact target repository/Issue identities and approved draft/final body digests in mutation and complete readback logs;
- a globally ordered call log for the one create, reconciliation polls, relationship operations, one confirmed final-body PATCH when a unique draft exists, and final convergence readback; the PATCH must be the last modeled mutation;
- successful create plus readback (`created`);
- timeout-after-success and delayed indexing converging to one match (`matched`), including an ambiguous final PATCH reconciled by exact readback;
- partial relationship success/failure without rollback;
- zero marker matches (`failed`, requiring a fresh preview and confirmation);
- duplicate observations that remain `ambiguous` even if a later readback has one match;
- rejected traces for blind create/PATCH retries, cross-repository matches, unapproved bodies, and final bodies that appear without the confirmed PATCH or before its phase.

Run from the repository root:

```bash
python3 -I acceptance/github_publication_fault_harness.py \
  --fixture acceptance/fixtures/github-publication-faults.json
```

This is a reproducible host-policy fault harness. It makes no HTTP request, uses no GitHub token, and is **not** live GitHub.com G evidence or a model transcript H. Live fixture instructions are in `../live-github/README.md`.
