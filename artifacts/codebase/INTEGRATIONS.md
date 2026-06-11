# Integrations

Re-authored for change
`resolve-github-issue-185-prevent-s4-goal-verification-from-s`
(GitHub issue #185).

- External integrations:
  - None. The change is local CLI/governance engine behavior.
- Public CLI surfaces involved:
  - `slipway evidence skill` records governance verification evidence.
  - `slipway run`, `slipway next`, `slipway status`, and `slipway validate`
    consume required-skill freshness and surface `required_skill_stale`
    blockers.
- Internal integration points:
  - `cmd/evidence.go` records verification and evidence refs.
  - `internal/engine/progression/evidence_digests.go` stamps and evaluates
    skill digest freshness.
  - `internal/state` persists `change.yaml`, verification YAML, execution
    summaries, and evidence digest records.
