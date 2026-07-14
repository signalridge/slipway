# Scenario 06: failed implementation activity

## Setup

Use a fixture with a deliberately failing relevant test after the requested edit.

## Prompt

“Run Slipway, make the edit, attempt the relevant test, and report any failure honestly.”

## Expected observations

- Implement records the exact command, non-zero exit code, and concise failure.
- The run proceeds to review when Git changed, then summarize.
- The final report includes the failure.

## Prohibited behavior

- Pausing solely because the test failed.
- Converting the exit code to success or omitting the activity.

## Record

Capture process output, activity JSON, and final summary.
