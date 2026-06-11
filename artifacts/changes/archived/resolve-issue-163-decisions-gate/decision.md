# Decision

## Alternatives Considered
Option A: add status blockers only in `DecisionContractBlockers`.
This would make `validate` and planning readiness fail closed, but
`cmd/next_skill.go` would still have its own string-only parser and could drift.

Option B: add status blockers only in `cmd/next_skill.go`.
This would prevent dead selected approaches from reaching host skills, but the
core decision contract and planning readiness would still treat the file as
usable.

Option C: add a shared parsed decision contract in `internal/engine/artifact`
and consume it from both progression readiness and next-skill constraints.
This is the selected direction because it gives one status taxonomy, one parser,
and one fail-closed decision authority.

## Selected Approach
Implement a shared parsed decision contract in `internal/engine/artifact`.
The parser will expose normalized status, whether a status was explicitly
provided, selected decision items, and status blockers. `DecisionContractBlockers`
will append status blockers at plan-audit and later, while `cmd/next_skill.go`
will stop surfacing pending or locked decisions when the parsed decision status
is rejected. Missing status remains compatible; explicit superseded,
deprecated, rejected, or unknown statuses fail closed.

## Interfaces and Data Flow
`decision.md` flows into `internal/engine/artifact.ParseDecisionContract`.
Progression uses the parsed contract through `DecisionContractBlockers` before
normalizing reason specs. The CLI next-skill path uses the same parsed contract
through `parseDecisionItems`, returning nil when the status is unusable so host
skills do not build on dead decisions. New canonical reason codes flow through
`internal/model/reason_code.go` and recovery guidance through
`internal/model/recovery.go`.

## Rollout and Rollback
Roll forward by adding parser/status tests first, then wiring progression and
next-skill consumers. Verify with targeted package tests followed by
`go test -count=1 ./...` and governed validation. Rollback is a normal git
revert of the parser, reason-code, and consumer changes; re-run the same tests
to confirm the prior decision contract behavior is restored.

## Risk
Main risk is compatibility: existing authored decisions have no status section.
The mitigation is to treat missing status as compatible and only fail closed for
explicit dead or unknown statuses. Secondary risk is parser drift between
readiness and host handoff; the shared parsed contract avoids that. A smaller
risk is overmatching status prose, so status parsing is limited to explicit
status-like headings.
