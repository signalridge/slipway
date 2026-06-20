# Intent

## Summary
harden execution auto mode

## Complexity Assessment
complex

This touches Slipway's governed execution auto-mode boundary, lifecycle audit
events, generated command surfaces, README safety text, and confirmation
requirement tests. The change is not a current vulnerability fix, but it hardens
auditing, future skill-boundary defaults, and regression coverage around safety
red lines.

## Guardrail Domains
None.

## In Scope
- Distinguish auto-acknowledged non-guardrail `human_verify` checkpoint
  resolution events from operator-provided `--resume-response` events.
- Keep `slipway learn` from treating auto-acknowledged checkpoint resolutions as
  indistinguishable human verification signals.
- Replace skill auto-softening's single-skill blocklist with an explicit
  allowlist of pure-pacing skills that are safe for `execution.auto`.
- Add regression tests for README and run command auto-mode safety red lines.
- Add a regression test for auto-off plus non-pacing blocker precedence over a
  handoff boundary.

## Out of Scope
- Do not weaken or rewrite the existing `security-review` hard-stop behavior;
  C2 is correct fail-closed behavior and remains unchanged.
- Do not add broad new lifecycle UX fields unless they are required to make the
  audited auto-acknowledgment distinguishable.
- Do not refactor unrelated lifecycle, review, or evidence gates.

## Constraints
- Preserve existing manual `--resume-response` behavior and event shape except
  where auto acknowledgments need an explicit marker.
- Preserve evidence gates: auto mode may only soften pacing, never required
  evidence or guardrail boundaries.
- Keep the implementation small and test-driven around the reported gaps.

## Acceptance Signals
- `go test ./cmd` covers auto-ack audit event distinction and C1 blocker
  precedence.
- `go test ./internal/toolgen` covers run command registry redline text.
- `go test ./internal/tmpl` covers run prompt redline text.
- `go test ./...` and `go build ./...` pass, or any unrelated known slow test is
  identified precisely.

## Open Questions
None.

## Deferred Ideas
- Broader observability fields for effective auto mode in top-level JSON can be
  considered separately if operators still need it after auto checkpoint events
  become auditable.

## Approved Summary
The user explicitly requested a new change to repair all confirmed issues from
the combined review: M1, M2, M3, and C1. This change hardens execution auto mode
by making auto `human_verify` checkpoint acknowledgments auditable, switching
skill auto-softening to an explicit safe allowlist, and adding regression pins
for README/run redlines plus auto-off blocker precedence. C2 is excluded because
the existing `security-review` hard-stop behavior is correct and should remain
unchanged.
