# Intent

## Summary
Make `validate --change <slug>` fail closed when the explicit slug does not
resolve to an active or archived governed change.

## Complexity Assessment
simple

The reproduction is a public CLI contract defect with a narrow behavioral
surface. It needs black-box command tests and a small resolver-path repair, not
new architecture or discovery.

## Guardrail Domains
None.

## In Scope
- `validate --change <missing> --json` must return a typed
  `change_not_found` error with exit code 3 instead of falling through to
  generic diagnostics.
- Explicit archived slug behavior for `validate --change <archived> --json`
  must remain fail-closed and documented by tests.
- Unscoped no-active `validate --json` semantics must be fixed in tests so the
  product contract is explicit.
- The fix may touch the active-change resolver and validate command boundary
  only as needed to align `validate` with the existing `status`, `next`,
  `done`, and `evidence` resolver behavior.

## Out of Scope
- Full `InvocationRoute` consolidation from opt.md section 1.1.
- Freshness field splitting from opt.md section 1.3.
- S3 action-contract alignment from opt.md section 1.4.
- Host capability/delegation reporting from opt.md section 1.5.
- Any release, supply-chain, or branch-protection work from opt.md section 2.

## Constraints
- Preserve unrelated dirty files in the root checkout (`.gemini/`,
  `coverage.out`, and `opt.md`).
- Use the bound worktree
  `.worktrees/fail-closed-missing-change` as the active change authority.
- Keep remediation output executable and stable for agents.

## Acceptance Signals
- `go run . validate --change definitely-not-a-change --json` exits 3 and
  returns `error_code: change_not_found`, `exit_code: 3`, and executable
  remediation.
- A black-box test covers explicit missing slug behavior.
- A black-box test covers explicit archived slug behavior.
- A black-box test fixes unscoped no-active `validate --json` behavior.
- Targeted tests for the changed command/resolver surface pass.

## Open Questions
None.

## Deferred Ideas
- General route model extraction remains deferred to opt.md section 1.1.
- Freshness/action contract cleanup remains deferred to opt.md sections 1.3
  and 1.4.

## Approved Summary
Approved under the user's standing auto-permit instruction for the opt.md
execution loop on 2026-06-26. This change is limited to making explicit missing
`validate --change <slug>` requests fail closed with `change_not_found` and exit
3, while locking explicit archived and unscoped no-active validation semantics
with tests.
