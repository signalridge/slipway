# Integrations

Re-authored for change `resolve-github-issue-156-add-a-change-implies-evidence-gate`
(GitHub issue #156).

- No external network, database, or service integration is expected.
- Public CLI integration surfaces:
  - `slipway evidence task` records runtime task evidence consumed by execution
    summary and sensitive-evidence readiness.
  - `slipway evidence skill` records governance skill verification consumed by
    plan/review/verify gates.
  - `slipway validate`, `slipway next`, and `slipway run` consume the same
    readiness result so read-only and mutating surfaces agree.
- Internal integration surfaces:
  - `internal/engine/progression` integrates execution-summary freshness,
    scope-contract, sensitive-evidence, stale-evidence recovery, and lifecycle
    advancement.
  - `internal/toolgen` and `internal/tmpl/templates/_partials` publish the
    command metadata and prompt body for the evidence command family.
