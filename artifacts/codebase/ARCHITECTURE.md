# Architecture

Re-authored for change `explain-domain-review-mapping` (GitHub issue #203).

Question: where should Slipway preserve and expose the evidence source that satisfies a governance control?

## Affected Seams

- `internal/engine/governance/actions.go:7-14` defines `RequiredAction`. It currently stores `control_id`, `mode`, `scope`, `description`, and `satisfied`, but no reason or evidence source for a satisfied action.
- `internal/engine/governance/actions.go:45-48` maps `domain-review` to `DomainReviewDone`. The policy decision is already abstracted as input to this layer.
- `internal/engine/governance/actions.go:73-85` defines `RequiredActionsInput`. It currently accepts boolean satisfaction inputs, not the source evidence that produced them.
- `internal/engine/governance/runtime_actions.go:15-20` defines runtime skill constants. `skillSpecComplianceReview` is the evidence skill used for domain review satisfaction.
- `internal/engine/governance/runtime_actions.go:39-44` computes review-side satisfaction. `domainReviewDone` comes from `skillSatisfied(skillSpecComplianceReview, ...)`, while independent review comes from `skillCodeQualityReview`.
- `internal/engine/governance/runtime_actions.go:64-75` passes only booleans into `ResolveRequiredActions`, losing the evidence-source mapping before CLI views are built.
- `cmd/governance_surface.go:42-56` converts `governance.RequiredAction` into CLI `governanceActionView`; it cannot expose information not present on the engine action.
- `cmd/status.go:94-99` defines the JSON shape used by status, validate, and next governance surfaces for each required action.

## Dependency Flow

`EvaluateGovernanceReadiness` computes runtime required actions, command builders map those actions through `cmd/governance_surface.go`, and `status`, `validate`, and `next --json --diagnostics` emit the resulting `required_actions` array. The issue #203 gap is not in control derivation; it is in runtime evidence attribution being reduced to a boolean before the JSON surface is rendered.

## Constraints And Invariants

- Preserve current policy semantics: `spec-compliance-review` is allowed to satisfy `domain-review` when the verification is passing and current for the latest execution summary.
- Keep stale or missing execution-summary behavior fail-closed; stale `spec-compliance-review` must not satisfy `domain-review`.
- Add JSON fields in an additive way so existing consumers of `control_id`, `mode`, `description`, and `satisfied` keep working.
- Keep action blockers based on unsatisfied blocking actions only; satisfied action explanations should not create new blockers.
