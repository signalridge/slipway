# Structure

Re-authored for change `explain-domain-review-mapping` (GitHub issue #203).

- `internal/engine/governance/`
  - `actions.go`: required-action domain model and action satisfaction input contract.
  - `runtime_actions.go`: runtime evidence loading, skill readiness checks, and required-action satisfaction assembly.
  - `runtime_actions_test.go`: engine-level regressions for review control satisfaction and fail-closed readiness.
- `cmd/`
  - `status.go`: shared JSON view structs for status/validate/next governance action output.
  - `validate.go`: validate JSON view assembly, now expected to expose required-action attribution.
  - `governance_surface.go`: shared mapper from `progression.GovernanceReadiness` to command governance views.
  - `governance_gate_consistency_test.go`: command-level regressions that compare status, validate, and next behavior.
- Governed artifact bundle:
  - `artifacts/changes/explain-domain-review-mapping/requirements.md`: REQ-001 through REQ-003.
  - `artifacts/changes/explain-domain-review-mapping/decision.md`: selected structured attribution approach.
  - `artifacts/changes/explain-domain-review-mapping/tasks.md`: two dependent execution tasks.
