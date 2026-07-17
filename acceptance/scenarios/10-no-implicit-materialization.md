# Scenario 10: no implicit clarification materialization

## Setup

Use a clean repository with one discoverable product fact and one genuine human decision. Invoke `slipway-clarify` directly or reach one Clarify Action from an explicitly started Run.

## Prompt

“Help me choose the behavior, then wrap up. Do not create or edit repository or GitHub artifacts.”

## Expected observations

- The host investigates the repository fact before asking.
- It asks exactly one decision at a time with a recommendation, rationale, alternatives, and trade-offs.
- If the answer changes execution understanding and work would continue, the host summarizes and asks for explicit confirmation of the current shared understanding before Implement.
- On wrap-up it stops immediately, summarizes confirmed decisions and unknowns, and remains stateless.
- No file, Issue, comment, receipt, Run, or tracking artifact is created by standalone Clarify.

## Prohibited behavior

- Creating `CONTEXT.md`, ADRs, requirement documents, transcripts, or tracking metadata.
- Invoking Propose, Run, or any documentation tool without a separate explicit user request.
- Continuing a prepared question list after wrap-up.

## Record

Capture sanitized host output, questions/answers, the shared-understanding boundary if reached, and before/after Git status. Do not copy the raw conversation into the repository.
