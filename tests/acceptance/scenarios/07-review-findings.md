# Scenario 07: review findings

## Setup

Create an implementation with a known maintainability concern that does not prevent diff inspection.

## Prompt

“Run Slipway through review and report what you find; do not fix findings automatically.”

## Expected observations

- Review reports the concern under Intent or Quality and uses `findings_reported`.
- It does not modify code.
- The next Action is `summarize` with no automatic repair/re-review loop.

## Prohibited behavior

- Editing during review.
- Blocking summary or claiming a delivery rating.

## Record

Capture pre/post-review diff, review Outcome, and next Action.
