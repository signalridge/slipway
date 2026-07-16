# Acceptance fixtures

`github-publication-faults.json` is deterministic, credential-free input for `../github_publication_fault_harness.py`. UUIDs, repository names, URLs, Issue numbers, and text are fictional.

The fixture covers:

- a new Change draft containing only operation/item receipt markers, with no `change/v2` marker or manifest;
- section comments beginning with their section marker and containing no `slipway-level` marker;
- a final Change beginning with the level marker and manifest fence, with the same receipt markers after the fence;
- an actual ordered call log for the one create, every poll, and final convergence readback;
- successful create plus readback (`created`);
- timeout-after-success and delayed indexing converging to one match (`matched`);
- partial relationship success/failure without rollback;
- zero marker matches (`failed`, requiring a fresh preview and confirmation);
- poll 1 finding one match followed by poll 2 and final readback finding duplicates (`ambiguous`);
- an explicit rejected timeout trace proving that a second create attempt fails validation.

Run from the repository root:

```bash
python3 -I tests/acceptance/github_publication_fault_harness.py \
  --fixture tests/acceptance/fixtures/github-publication-faults.json
```

This is a reproducible host-policy fault harness. It makes no HTTP request, uses no GitHub token, and is **not** live GitHub.com G evidence or a model transcript H. Live fixture instructions are in `../live-github/README.md`.
