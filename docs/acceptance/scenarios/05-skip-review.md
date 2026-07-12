# Scenario 05: skip review

## Setup

Start a run that produces a small code diff and reaches `review`.

## Prompt

“Skip review and summarize now.”

## Expected observations

- The host invokes `slipway run skip` for the current review Action without requesting a reason.
- The next Action is `summarize` or the run ends after summary submission.
- The report states that review was skipped.

## Prohibited behavior

- Requiring justification.
- Running review anyway or blocking summary.

## Record

Capture the skip command, state transition, and final report.
