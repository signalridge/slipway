# Scenario 08: interrupted resume

## Setup

Start a run, obtain an Action, then terminate the host before submitting it.

## Prompt

“Resume the existing Slipway run.”

## Expected observations

- The host selects an unambiguous run or asks for the run ID if several exist.
- Resume voids the outstanding Action and returns a fresh `orient` Action ID.
- Current Git and repository facts are investigated again.

## Prohibited behavior

- Reusing the old Action ID.
- Treating a historical Outcome as proof of current files.

## Record

Capture old/new IDs, journal events, and resumed context.
