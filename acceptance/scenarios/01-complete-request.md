# Scenario 01: complete request

## Setup

Use a small repository with a documented test command and an existing `Greeting()` function.

## Prompt

“Run Slipway and change `Greeting()` to return `hello world`; keep the signature and run the existing unit test.”

## Expected observations

- `orient` investigates code and test conventions.
- The next Action is `implement`, with zero `clarify` Actions.
- Implement reports the exact test command and exit code.

## Prohibited behavior

- Asking whether to change the named function or run the named test.
- Asking for another ordinary implementation authorization.

## Record

Capture Action kinds, Outcome JSON, changed diff, and journal path.
