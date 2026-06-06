# Tasks
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` (#92) Make assurance-coverage traceability gaps stage-aware. In
  `internal/engine/governance/traceability.go` add a `LifecycleState
  model.WorkflowState` field to `TraceabilityInput`, and in `EvaluateTraceability`
  compute `assuranceBlocking := !assuranceVerdictsExpectedLater(input.LifecycleState)`
  where a small helper returns true for `S0_INTAKE`/`S1_PLAN`/`S2_EXECUTE` (before
  review) and false otherwise (including empty/unknown → fail-closed). Apply that
  `Blocking` value to BOTH assurance gaps (`assurance verifies no requirement IDs`
  and `requirement missing assurance coverage verdict`); leave every other gap
  type unchanged. In `internal/engine/governance/health.go` pass
  `change.CurrentState` into the `TraceabilityInput` at the `deriveGovernanceControls`
  call site. Test-first: extend `internal/engine/governance/traceability_test.go`
  to cover S2 (non-blocking → WARN), unknown-state (blocking → FAIL), S3/S4/DONE
  (blocking → FAIL), and complete-coverage (OK at S2 and S3); update the existing
  `TestTraceabilityAssuranceMustCoverEveryRequirement` /
  `TestTraceabilityAssuranceNoREQIDs` to assert the fail-closed default.
  Then complete the same #92 behavior on the doctor-synthesis surface: in
  `cmd/health.go`, `governanceDoctorActions` MUST NOT promote a
  `traceability_coherence` check whose gaps are all non-blocking into a doctor
  action (add a `traceabilityCheckHasBlockingGap` guard); a blocking (`FAIL`)
  check still surfaces its action unchanged (REQ-004).
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/governance/traceability.go", "internal/engine/governance/traceability_test.go", "internal/engine/governance/health.go", "cmd/health.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004]

- [x] `t-02` (#92) Prove the health surface and lock the suite. Add a
  governance-health test in `internal/engine/governance/health_test.go` that
  builds a snapshot for a change at `S2_EXECUTE` with incomplete per-requirement
  assurance coverage and asserts `traceability_coherence` is `WARN` and the report
  is not unhealthy from that gap, plus a companion at `S3_REVIEW` asserting `FAIL`.
  Then add command-level doctor regressions in `cmd/health_test.go`: a
  deterministic unit test over `governanceDoctorActions` (non-blocking
  traceability check → no action; blocking → action present) plus an end-to-end
  `health --governance --json --doctor` test asserting S2 → WARN with no
  `governance_traceability_coherence` action and S3 → FAIL with the action
  present (REQ-004). Then run `go build ./... && go vet ./... && go test ./...`
  green and confirm no toolgen/generated-surface drift (no generated
  skill/command/doc surface changes).
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/governance/health_test.go", "cmd/health_test.go"]
  - task_kind: code
  - covers: [REQ-003]
