---
skill_id: ci-triage
domain: repair-ci
function: triage failing CI runs to root cause before retrying
tier: T2
primary_attachment: procedure
summary: "Use when CI is failing and a retry is being considered. Triggers on repair or status commands, or user text naming CI failures."
trigger_signals:
  - command: ["repair", "status"]
    reason: "repair or status command invoked; CI failures may be in scope"
  - user_text_matches: ["ci failing", "ci broken", "build failing", "pipeline failing"]
    reason: "User text names a CI failure"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: repair
    attachment: procedure
  - type: command-auto
    target: status
    attachment: checklist
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

## Scripts
- `scripts/fetch-pr-checks.py` — fetch CI check-run status for a PR and
  extract failure log snippets. Read-only. Requires the `gh` CLI on
  `PATH` plus `GH_TOKEN` (or `GITHUB_TOKEN`, or a prior `gh auth
  login`); the helper fails fast with a credential-error message when
  credentials are missing or rejected.
