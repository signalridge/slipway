# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: 
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: 

## Summary
Complete issue #53 Tier 2 evidence contract and closeout reuse fixes
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Add a supported task evidence recording surface, targeted as a CLI command such as `slipway evidence task`, so governed host skills do not hand-write `.git/slipway/runtime/changes/<slug>/evidence/tasks/*.json`.
- Ensure the task evidence surface controls runtime fields such as run-summary version, capture timestamp, freshness inputs, and fail-closed verdict semantics.
- Update the wave-orchestration host/skill contract so wave execution records runtime task evidence through the supported surface before execution-summary generation.
- Update final-closeout semantics and host guidance to permit a freshness-attested reuse branch only when goal-verification evidence matches the current run version, freshness inputs, and content state.
- Add focused regressions covering task-evidence recording, execution-summary rebuild/repair behavior, and final-closeout reuse gating.

## Out of Scope
- Tier 1 issue #53 workflow/diagnostic fixes; PR #60 already merged those changes.
- Tier 3 stale-planning recovery state-machine work, including S3/S4 plan-audit refresh routing and pivot/rescope semantics.
- Broad repair of unrelated archived bundles or sibling worktree schema drift, except where a Tier 2 regression needs a narrow fixture.
- Any `slipway done` finalization for this Tier 2 change; stop at done-ready/close-ready per user instruction.

## Constraints
- Preserve fail-closed governance semantics: missing or mismatched runtime evidence must block reuse or summary generation instead of silently passing.
- Keep the implementation split aligned with GitHub issue #53 comment `4604547893`: this PR addresses root causes A and F only.
- Prefer existing state/progression model helpers and command patterns over ad hoc JSON path writes.
- Avoid disturbing unrelated local dirt in the root checkout and sibling worktrees.

## Acceptance Signals
- A supported CLI task-evidence path can write runtime task evidence and execution-summary generation can consume it without manual JSON edits.
- `repair` behavior remains able to rebuild summaries when source task evidence exists, and clearly reports missing source task evidence as non-repairable when it does not.
- Final-closeout reuse accepts fresh goal-verification only when run version, freshness inputs, and content state match, and rejects stale/mismatched proof.
- Focused Go tests plus `go test ./...`, `go build ./...`, `go vet ./...`, `git diff --check`, and `slipway validate --json` pass before stopping short of `slipway done`.

## Open Questions
- None.

## Deferred Ideas
- Non-destructive S3/S4 stale-planning recovery and plan-audit refresh routing remain Tier 3.
- Cleanup of unrelated old archived bundles carrying retired `worktree_path` fields remains separate unless elevated by a focused Tier 2 test.

## Approved Summary
User request on 2026-06-03: complete issue #53 Tier 2 fixes and development, stopping before closeout finalization. This change implements the runtime task evidence contract and final-closeout goal-verification reuse path for root causes A and F from https://github.com/signalridge/slipway/issues/53#issuecomment-4604547893. It excludes Tier 1, Tier 3, unrelated archive cleanup, and any final `slipway done` for the Tier 2 governed change.
