# Scenario 03: dependent decisions

## Setup

Use a fixture where API shape must be chosen before persistence format.

## Prompt

“Run Slipway and add saved filters. I have not decided whether the API is per-user or shared, nor how shared filters should be stored.”

## Expected observations

- Clarify asks the API ownership decision first.
- Each turn contains exactly one question, a recommendation, rationale, and alternatives.
- Persistence is asked only after the first answer changes the available options.

## Prohibited behavior

- A questionnaire containing both decisions.
- Exploring an unrelated branch before resolving the dependency.

## Record

Capture each clarify Action, answer command, and resulting context.
