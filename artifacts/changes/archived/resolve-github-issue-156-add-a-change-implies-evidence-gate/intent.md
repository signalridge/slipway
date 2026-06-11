# Intent

## Summary
Resolve GitHub issue #156: add a change-implies-evidence gate

## Complexity Assessment
complex

The change touches governed readiness, execution-summary interpretation,
reason-code contracts, and recovery guidance. It needs regression coverage and
governed review because it changes a public lifecycle gate.

## Guardrail Domains
external_api_contracts

## In Scope
- Add a readiness gate that blocks sensitive changed files when the execution
  evidence lacks the category-specific owning proof.
- Cover schema migration, auth/authz, and API contract file categories in the
  initial built-in rule set.
- Report a canonical reason code and recovery guidance that points operators to
  task evidence markers instead of bypasses.
- Keep the new gate independent from freshness and scope-contract readiness.
- Update the codebase map for the issue #156 planning context.

## Out of Scope
- A complete ecosystem catalog for every possible sensitive file convention.
- A force flag, environment-variable bypass, or compatibility path that disables
  sensitive evidence enforcement.
- Replacing scope-contract target-file checks or review-stage skill evidence.

## Constraints
- The gate must use the current execution summary and changed-file surfaces.
- A sensitive-domain miss must fail closed with a stable reason code.
- Evidence markers must be explicit: `migration-applied`, `auth-review`, and
  `contract-test`.
- Tests must prove both blocked and passing paths.

## Acceptance Signals
- Sensitive changed files without owning evidence produce
  `sensitive_evidence_missing`.
- Matching task evidence markers clear the blocker.
- Recovery guidance names the evidence command and required markers without
  documenting a bypass.
- Targeted package tests and `go test ./...` pass.
- `slipway validate --json --change resolve-github-issue-156-add-a-change-implies-evidence-gate`
  reports fresh governed readiness with review skill evidence present.

## Open Questions
None

## Deferred Ideas
- Configurable sensitive-evidence rules can be added after this built-in gate is
  stable.

## Approved Summary
Approved 2026-06-10: implement the issue #156 change-implies-evidence gate with
category-specific task evidence markers, no bypass, focused tests, and governed
review evidence.
