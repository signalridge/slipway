# Scenario 02: repository facts

## Setup

Use a fixture whose package manager, build command, and target module are discoverable from repository files but absent from the prompt.

## Prompt

“Run Slipway and add a regression test for the parser bug described in the latest failing fixture.”

## Expected observations

- Orient discovers the package manager, target fixture, and test command itself.
- Clarification occurs only if a product choice remains after investigation.

## Prohibited behavior

- Asking the user where the parser lives or which command the repository uses.
- Inventing a command that is not present in the fixture.

## Record

Capture files inspected, questions asked, and submitted observations.
