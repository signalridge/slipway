# Intent

## Summary
Resolve GitHub issue #160: freeze reason-code taxonomy with a snapshot test and add a test lint that bans assertions against ReasonCode/JSON message prose.
## Complexity Assessment
simple
The change is a focused contract/test-quality hardening task in the Go codebase. It does not introduce a new user workflow, external API, storage migration, or sensitive-domain behavior.

## In Scope
- Add a freezing/snapshot-style regression for the canonical reason-code taxonomy so additions/removals are explicit.
- Add a test lint/contract check that rejects tests asserting against unstable `Message` prose for reason/error contract payloads.
- Update existing tests caught by that lint to assert stable fields such as `Code`, `Detail`, `ErrorCode`, or structured collections.
- Keep implementation scoped to issue #160's reason-code taxonomy and message-prose assertion contract.

## Out of Scope
- Do not implement issue #152 or touch the context-pressure hook active change.
- Do not redesign the full JSON error schema.
- Do not mass-rename existing reason codes unless required by the freeze test.
- Do not add an external lint dependency when a repo-local Go test can enforce the contract.

## Constraints
- Preserve existing operator-facing messages as presentation text; the new contract is about tests and machine-stable reason codes.
- Prefer repo-native Go tests and current package layout.
- Use the current governed worktree behavior as the lifecycle authority.

## Acceptance Signals
- A regression proves the current taxonomy is not frozen and the current message-prose assertions are detectable.
- `go test -count=1 ./...` passes after the fix.
- `go run . validate --json` reaches a done-ready ship gate state for this governed change.

## Open Questions
None

## Approved Summary
Confirmed by user on 2026-06-10T02:17:02Z. Resolve #160 by freezing the canonical reason-code taxonomy with an explicit regression and adding a repo-local test lint that bans assertions against unstable `Message` prose for reason/error contract payloads. Keep the change limited to the taxonomy/test-contract surface, excluding #152, full JSON schema redesign, broad reason-code renaming, and new external lint tooling unless unavoidable. Completion requires the targeted regressions and full Go test suite to pass, then Slipway validation to reach done-ready.
