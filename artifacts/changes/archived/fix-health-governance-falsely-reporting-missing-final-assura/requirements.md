# Requirements
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Assurance-coverage traceability gaps are stage-aware before review (#92)
REQ-001: Before `S3_REVIEW` (i.e. at `S0_INTAKE`, `S1_PLAN`, or `S2_EXECUTE`), the
traceability evaluator MUST classify the per-requirement assurance gaps
(`requirement missing assurance coverage verdict` and `assurance verifies no
requirement IDs`) as **non-blocking** so that `health --governance` reports
`traceability_coherence` as `WARN` (not `FAIL`) and `governance.healthy` is not
driven false by these gaps alone. The lifecycle state MUST flow into the
evaluator from the owning call site rather than being inferred; when the state is
unknown/empty the gaps MUST stay blocking (fail-closed default). No other
traceability gap type changes classification.

#### Scenario: S2 execution with incomplete assurance coverage is a warning
GIVEN a change at `S2_EXECUTE` whose requirements include REQ IDs not yet covered
by an assurance coverage verdict
WHEN governance health is computed for that change
THEN `traceability_coherence` is `WARN`, the assurance gaps are present but
non-blocking, and the overall report is not marked unhealthy by those gaps.

#### Scenario: Unknown lifecycle state stays fail-closed
GIVEN `EvaluateTraceability` is called without a lifecycle state (empty/unknown)
and assurance coverage is incomplete
WHEN traceability is evaluated
THEN the assurance gaps remain blocking and the status is `FAIL`.

### Requirement: Assurance coverage still fails closed at review and closeout (#92)
REQ-002: At `S3_REVIEW`, `S4_VERIFY`, and `DONE`, the same assurance-coverage
gaps MUST remain blocking, so `traceability_coherence` is `FAIL` and a change
cannot reach `done` while any requirement lacks an assurance coverage verdict.
A change whose assurance covers every requirement MUST produce no assurance gap
at any state.

#### Scenario: S3 review with incomplete assurance coverage blocks
GIVEN a change at `S3_REVIEW` whose requirements include REQ IDs not covered by an
assurance coverage verdict
WHEN traceability is evaluated
THEN the assurance gaps are blocking and the status is `FAIL`.

#### Scenario: Complete assurance coverage is OK regardless of state
GIVEN a change whose assurance verifies every requirement ID
WHEN traceability is evaluated at `S2_EXECUTE` or at `S3_REVIEW`
THEN no assurance gap is produced and the status is `OK`.

### Requirement: Readiness surfaces agree and the suite stays green (#92)
REQ-003: The fix MUST be limited to the governance traceability evaluator, its
health call site, and the doctor-synthesis surface that renders governance
health checks, plus tests; `slipway validate` / `slipway next` behavior is
unchanged (they already route an S2 change to `wave-orchestration`), so all three
readiness surfaces agree on the next action at S2. `go build ./...`,
`go vet ./...`, and `go test ./...` MUST pass, and no generated skill/command/doc
surface changes (no toolgen drift introduced).

#### Scenario: Build, vet, and tests pass with no surface drift
GIVEN the change is implemented
WHEN `go build ./... && go vet ./... && go test ./...` runs
THEN all pass, and `validate`/`next` outputs for an S2 change are unchanged
(next action remains `wave-orchestration`).

### Requirement: Doctor surface raises no non-repairable incident for advisory traceability (#92)
REQ-004: The `health --governance --doctor` surface MUST treat a
`traceability_coherence` check whose gaps are all non-blocking (advisory, e.g.
pre-review assurance coverage verdicts) as carrying no actionable repair: the
doctor view MUST NOT emit a `governance_traceability_coherence` action for it, so
`--doctor` raises no non-repairable incident that contradicts `validate`/`next`.
When the same check is blocking (`FAIL`, i.e. at/after `S3_REVIEW`), the doctor
view MUST still emit the `governance_traceability_coherence` action. No other
governance check's doctor behavior changes.

#### Scenario: S2 advisory traceability raises no doctor action
GIVEN a change at `S2_EXECUTE` with incomplete per-requirement assurance coverage
WHEN `slipway health --governance --json --doctor` runs
THEN `traceability_coherence` is `WARN` and the doctor view contains no
`governance_traceability_coherence` action.

#### Scenario: S3 blocking traceability still raises a doctor action
GIVEN a change at `S3_REVIEW` with incomplete per-requirement assurance coverage
WHEN `slipway health --governance --json --doctor` runs
THEN `traceability_coherence` is `FAIL` and the doctor view contains a
`governance_traceability_coherence` action.
