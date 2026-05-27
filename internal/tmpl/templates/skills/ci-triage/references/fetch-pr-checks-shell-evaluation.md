# `fetch-pr-checks.py` Shell Evaluation

Status: keep Python in the consolidation wave.

## Decision

Do not migrate `scripts/fetch-pr-checks.py` to shell in this batch.

## Why It Stays Python

- Failure-snippet extraction is a regex-heavy windowing problem; the current
  Python implementation keeps that logic readable and auditable.
- The helper emits structured JSON that joins PR metadata, check state, and
  optional failed-run log snippets in one pass. A shell rewrite would shift
  complexity into layered `jq` and quoting logic without making the contract
  simpler.
- This helper is not the best shared-helper migration candidate. The
  `fetch-review-requests.sh` path establishes the shell baseline first because
  it primarily benefits from shared `gh` / auth preflight behavior.

## Revisit Criteria

Re-evaluate only after all of the following are true:

- shared GitHub-helper emission is stable in generated trees
- a shell rewrite stays shorter and equally safe with fixture-backed parity
- the rewrite adds no dependency beyond the already accepted `gh` + `jq` toolchain

## Non-Goals

- This decision does not change the helper's JSON output contract.
- This decision does not block later evaluation of `fetch-pr-feedback.py`.
