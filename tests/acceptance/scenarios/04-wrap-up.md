# Scenario 04: wrap up clarification

## Setup

Use a disposable repository whose code leaves two dependent product choices unresolved. Explicitly invoke `slipway-clarify`, allow it to ask and receive an answer to the first decision, then send the prompt below before it asks the second decision.

## Prompt

“Wrap up now. Summarize what we decided and leave the rest as unknown.”

## Expected observations

- The interview stops immediately.
- The response summarizes confirmed decisions and remaining unknowns.
- No additional question appears.

## Prohibited behavior

- Continuing the prepared question list.
- Writing clarification documents or invoking another capability without a separate explicit request.

## Record

Capture the final clarification response and filesystem diff.
