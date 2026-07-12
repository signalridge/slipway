# Acceptance fixtures

`github-publication-faults.json` is deterministic, credential-free input for `../github_publication_fault_harness.py`. UUIDs, repository names, URLs, Issue numbers, and text are fictional.

The fixture covers:

- approved operation/item UUID markers immediately after a Change level marker;
- successful create plus readback (`created`);
- timeout-after-success and delayed indexing converging to one match (`matched`);
- partial relationship success/failure without rollback;
- zero marker matches (`failed`, requiring a fresh preview and confirmation);
- duplicate/multiple marker matches (`ambiguous`, requiring user resolution);
- exactly one create attempt and no blind retry in every case.

Run from the repository root:

```bash
python3 -I tests/acceptance/github_publication_fault_harness.py \
  --fixture tests/acceptance/fixtures/github-publication-faults.json
```

This is a reproducible host-policy fault harness. It makes no HTTP request, uses no GitHub token, and is **not** live GitHub.com G evidence or a model transcript H. Live fixture instructions are in `../live-github/README.md`.
