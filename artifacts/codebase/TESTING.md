# Testing

Re-authored for change `explain-domain-review-mapping` (GitHub issue #203).

## Existing Coverage

- `internal/engine/governance/runtime_actions_test.go:128-138` verifies that stale `spec-compliance-review` does not satisfy `domain-review`, and that current `code-quality-review` can satisfy `independent-review`.
- `cmd/governance_gate_consistency_test.go:127-167` verifies that missing execution-summary blockers are consistent across status, validate, and next when review evidence is present but not ready.
- `cmd/governance_gate_consistency_test.go:392-409` verifies that next surfaces `domain-review` blockers when a guardrail domain is active and no satisfying evidence exists.
- `cmd/governance_surface.go:42-56` is a narrow mapping seam suitable for a focused unit-style assertion through existing command-view helpers.

## Gaps For Issue #203

- No test asserts that a satisfied `domain-review` action explains that it was satisfied by `spec-compliance-review`.
- No test asserts that the explanation survives the command view mapping used by status, validate, and next.
- Existing stale-evidence tests check failure diagnostics, but not positive traceability for satisfied controls.

## Verification Plan

- Add an engine-level regression in `internal/engine/governance/runtime_actions_test.go` for `domain-review` satisfied by `spec-compliance-review`, including a stable evidence-source field.
- Add or extend a command-level regression in `cmd/governance_gate_consistency_test.go` to verify status, validate, and next all expose the satisfied-by mapping for the same runtime action.
- Run `go test -count=1 ./internal/engine/governance ./cmd` after implementation, then broaden as governed closeout requires.
