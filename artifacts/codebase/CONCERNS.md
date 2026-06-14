# Concerns

Re-authored for change `explain-domain-review-mapping` (GitHub issue #203).

- Traceability risk: preserving only `Satisfied bool` makes a control appear to satisfy itself when another review skill is the actual evidence source.
- Compatibility risk: changing existing JSON fields would break consumers; prefer additive fields such as `satisfied_by`.
- False-positive explanation risk: an explanation must only appear when the action is actually satisfied by current passing evidence. Stale or missing run-summary cases must remain unsatisfied and continue to show diagnostics.
- Scope risk: requiring a new `domain-review.yaml` would change policy semantics and expand the issue beyond user-facing traceability unless code proves the current mapping is wrong.
- Surface consistency risk: status, validate, and next share `cmd/governance_surface.go`; fixing only one command would leave the black-box behavior inconsistent.
