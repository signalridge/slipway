---
skill_id: ci-triage
domain: repair-ci
function: triage failing CI runs to root cause before retrying
tier: T2
primary_attachment: procedure
summary: "Use when CI, a build, or a pipeline is failing and a retry is being considered. Triggers on the `slipway fix` command or user text naming CI/build/pipeline failures."
trigger_signals:
  - command: fix
    reason: "fix command invoked; CI failures may be in scope"
  - user_text_matches: ["ci failing", "ci broken", "build failing", "pipeline failing"]
    reason: "User text names a CI failure"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: fix
    attachment: procedure
---

# CI Triage

```
IRON LAW: NO RETRY WITHOUT A CLASSIFIED ROOT CAUSE
```

## Purpose
Failing CI is a signal, not a nuisance. Retrying without classifying the
failure wastes time and normalizes flakes. Classify every red run, then act.

## Procedure
1. Pull the failing log; quote the exact error line(s) with file:line.
2. Classify the failure as one of: `code-fault` (the change broke behavior),
   `test-fault` (the test is wrong or brittle), `infra-fault` (runner,
   network, timeout), `flake` (intermittent, non-deterministic), or
   `env-drift` (dependency or config change outside the PR).
3. For `code-fault` / `test-fault`: fix and re-run.
4. For `infra-fault`: retry is acceptable; record that the retry succeeded
   and link the infra incident if known.
5. For `flake`: open a flake ticket with the quoted failure; do not retry
   silently. Bounded flake budgets belong in the ticket, not the PR.
6. For `env-drift`: identify the drift source; pin or rollback. Do not patch
   the test to hide the drift.

## Checklist
- [ ] Exact failure line quoted with file:line.
- [ ] Failure classified before any retry.
- [ ] Retries happen only for `infra-fault`.
- [ ] Flakes are ticketed, not silently retried.
- [ ] Env drift is pinned or rolled back, not masked.

## Anti-patterns
- "Retry the PR" as a first response.
- "Flaky test; skip it" without a ticket and a budget.
- Masking env drift by pinning the broken test.

## Helpers
- `slipway tool fetch-pr-checks --repo owner/repo --pr N` — fetch CI
  check-run status, failed check annotations, and check-run output summaries
  for a PR. Read-only.
  Defaults to `--backend auto`: use authenticated `gh` when available, and use
  the token-backed API when `gh` is unavailable or reports an auth-required
  error while `GH_TOKEN` or `GITHUB_TOKEN` is set. Use `--backend gh` to require
  GitHub CLI, or `--backend api` to require token API.
  The helper fails closed when no authenticated backend is available. No
  generated Python, shell, or `jq` helper script is required. See
  `references/fetch-pr-checks-shell-evaluation.md` for the retirement note.
