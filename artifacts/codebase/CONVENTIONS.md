# Conventions

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- Gate-bearing readiness checks should use canonical `model.ReasonCode` values,
  not free-form blocker prose.
- Recovery text must name a legal public command for the current lifecycle
  position. For `sensitive_evidence_missing`, recovery starts with
  `slipway run` so the workflow stays in or reopens to S2 before using
  `slipway evidence task`.
- Runtime task evidence is recorded through `slipway evidence task`; governance
  skill verification is recorded through `slipway evidence skill`. Do not
  hand-edit engine-owned freshness state or verification records as a normal
  workflow path.
- Generated command surfaces are backed by `internal/toolgen/toolgen.go` and
  command body templates. When a Cobra flag is added, update the generated
  command metadata in the same change.
- Sensitive evidence markers are lowercase hyphenated prefixes such as
  `migration-applied`, `auth-review`, and `contract-test`.
