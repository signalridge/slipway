# Research

## Alternatives Considered

### Architecture

- Affected modules: `internal/engine/governance/actions.go`, `internal/engine/governance/runtime_actions.go`, `cmd/governance_surface.go`, `cmd/status.go`, plus engine and command regressions.
- Dependency chains: runtime verification records are loaded in `runtime_actions.go`, converted into `governance.RequiredAction`, passed through `progression.GovernanceReadiness`, then rendered by `cmd/governance_surface.go`.
- Blast radius: low to medium. The change is additive JSON metadata on governance required actions, but those actions are part of user-facing CLI contracts.
- Constraints: `domain-review` must remain fail-closed on missing, failing, stale, or execution-summary-unbound `spec-compliance-review` evidence.

### Patterns

- Existing conventions: `RequiredAction` is the engine-owned action model, and `governanceActionView` is the command-owned JSON view.
- Reusable abstractions: `runtimeVerificationState.skillSatisfied` is the existing source of evidence readiness; extending the result shape there avoids duplicating readiness logic in command code.
- Convention deviations: no separate `domain-review` verification skill exists today, so the explicit mapping must say that the control is satisfied by the `spec-compliance-review` skill rather than inventing a new evidence file.

### Risks

- Technical risks: medium compatibility risk if JSON shape is changed non-additively; low runtime risk if fields are additive.
- Guardrail domains: `external_api_contracts`, because `required_actions` is a machine-readable CLI surface.
- Reversibility: high. The fix can be reverted by removing additive fields and tests without data migration.

### Test Strategy

- Existing coverage: stale/missing review evidence blockers are covered, but positive satisfied-by traceability is not covered.
- Infrastructure needs: no new fixtures are required; existing helpers can create governed changes, execution summaries, and skill verification records.
- Verification approach: assert engine action metadata and command view JSON metadata for a change where only `spec-compliance-review` evidence satisfies `domain-review`.

### Options

- Option A: Add an explicit evidence attribution field to `governance.RequiredAction`, populate it in runtime action resolution, and map it through the command view. Tradeoff: slightly expands the engine model, but preserves source-of-truth attribution across all CLI surfaces.
- Option B: Append a textual phrase to `Description` when `domain-review` is satisfied. Tradeoff: minimal structure change, but weak for JSON consumers and easy to confuse with remediation text.
- Option C: Require a separate `domain-review` evidence record. Tradeoff: makes the black-box trail obvious, but changes current policy semantics and adds workflow friction beyond the issue's expected behavior.
- Selected: Option A. It directly fixes traceability without changing policy semantics, and it gives scripts a structured field rather than requiring description parsing.

## Unknowns

- Resolved: Is `spec-compliance-review` intended to satisfy `domain-review`? Yes. Runtime action resolution computes `domainReviewDone` from `skillSpecComplianceReview`, while stale or unready records fail closed.
- Resolved: Where is the explanation lost? It is lost before rendering because `RequiredActionsInput` carries only booleans and `governanceActionView` exposes no satisfied-by field.
- Remaining: None.

## Assumptions

- Adding an optional JSON field is backward-compatible for current consumers. Evidence: existing action fields remain unchanged in `cmd/status.go`.
- A satisfied-by record should appear only for satisfied actions. Evidence: stale evidence currently changes `Satisfied` to false and appends diagnostics to `Description`.
- The issue is a traceability/UX defect, not a policy defect. Evidence: issue #203 states the reporter is not asserting the mapping itself is invalid, only that the surface does not explain it.

## Canonical References

- `internal/engine/governance/actions.go:7-14`
- `internal/engine/governance/actions.go:45-48`
- `internal/engine/governance/actions.go:73-85`
- `internal/engine/governance/runtime_actions.go:15-20`
- `internal/engine/governance/runtime_actions.go:39-44`
- `internal/engine/governance/runtime_actions.go:64-75`
- `internal/engine/governance/runtime_actions.go:195-209`
- `cmd/governance_surface.go:42-56`
- `cmd/status.go:94-99`
- `internal/engine/governance/runtime_actions_test.go:128-138`
- `cmd/governance_gate_consistency_test.go:127-167`
- `cmd/governance_gate_consistency_test.go:392-409`
