# `fetch-pr-checks` Script Retirement

Status: retired from generated skill scripts.

## Decision

Do not ship a shell or Python helper for CI check discovery. Use the compiled
Slipway helper:

```bash
slipway tool fetch-pr-checks --repo owner/repo --pr N
```

## Why It Moved Into Slipway

- Failure-signal extraction spans check conclusions, annotations, and output
  summaries; keeping the implementation in Go gives tests one runtime and one
  error taxonomy.
- The helper emits structured JSON that joins PR metadata, check state, and
  failure annotations/output summaries in one pass. Shell plus `jq` would shift
  complexity into quoting and tool availability instead of simplifying the
  contract. Full failed-run logs remain an explicit operator follow-up through
  the run URL or `gh run view --log-failed` when needed.
- GitHub access is backend-selected: `--backend auto` prefers authenticated
  `gh`, falls back to token API only when `gh` is unavailable or reports an
  auth-required error and `GH_TOKEN`/`GITHUB_TOKEN` is set, and fails closed
  when neither authenticated backend exists. Generated skills no longer ship
  Python, shell, or `jq` helper scripts for this helper.

## Operator Contract

- Missing or rejected GitHub authentication fails closed with a backend-specific
  remediation message.
- `--repo owner/repo` is required so the helper does not rely on git remotes or
  ambient CLI state.
- `--pr N` is required and must identify the pull request being triaged.

## Non-Goals

- This helper does not make retry decisions.
- This helper does not post comments or mutate PR state.
