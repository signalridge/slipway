---
name: slipway-review
description: Inspect intent and quality once and report findings without modifying code or creating a repair loop.
disable-model-invocation: true
---

# Slipway Review

Use this capability only for a current Review Action or when the user explicitly invokes it directly. It is always read-only.

For a Run Action, compare the current workspace with the immutable Run-start HEAD and fingerprint. List pre-existing dirty and untracked paths separately. A start-to-current difference is only an observation: concurrent user edits, other Runs, or tools may have contributed, so never claim that the current Run or Implement caused it merely because it appeared later.

Inspect two dimensions:

1. **Intent** — missing, partial, incorrect, excessive, or out-of-scope behavior relative to the pinned self-contained Requirements and confirmed decisions.
2. **Quality** — design, error handling, concurrency, security, maintainability, and test-sensitive concerns.

Report concrete findings with locations, details, and uncertainties. The strict Run Outcome must carry `action_kind: "review"` matching the current Action. Submit only these combinations:

- host `completed` with `review.result` `no_findings_reported` or `findings_reported`;
- host `partial` with `review.result` `inconclusive`;
- host `error` with `review.result` `error`.

Set `pause` and `implementation` to JSON `null`, include every common Outcome field, and leave `suggested_actions` empty. `not_run` is reserved for a CLI review-skip projection.

Do not modify files, return `needs_input`, ask the user to choose a repair, suggest Implement, issue pass/fail or approval language, pause the Run, or create an automatic repair or re-review loop. Findings always flow to Summary; the user may later choose a separate Change or a new Run.

When invoked directly outside a Run, use the baseline named by the user. If none is named, inspect the current workspace against `HEAD` and state that it is a standalone baseline, not a Run-start snapshot. Return the same factual review without inventing Run attribution or protocol IDs.
