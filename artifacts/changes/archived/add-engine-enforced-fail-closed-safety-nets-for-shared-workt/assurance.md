# Assurance

## Scope Summary

This change delivers engine-enforced fail-closed safety nets for shared-worktree
wave parallelism:

- C0 aligns `WaveDispatchParallel` with the host-written `parallel_subagents`
  dispatch token.
- C1 adds task changed-file scope-escape and parallel-wave changed-file overlap
  gates.
- C2 removes silent parallel-dispatch inference and blocks started parallel
  waves without explicit dispatch evidence.
- C3 requires one executor-agent handle per planned task for
  `parallel_subagents` waves.
- C4 aligns the generated wave-orchestration skill and executor-dispatch
  reference with the engine-enforced safety model.
- C5 adds non-blocking view-only wave-narrowing advisories, including the
  post-review repair that preserves explicit nested directory targets from
  equivalent raw `tasks.md` target text for advisory-only analysis.

All new gates flow through the existing `evaluateGovernedWaveExecution`
assembly path and `ExecutionSummary.OpenBlockers`; the engine records and gates
evidence but does not spawn executors.

## Verification Verdict

Current lifecycle evidence is run_summary_version=2. The active worktree has
passed the required review and verification path through S4:

- `wave-orchestration.yaml`: pass, run_version=2, with all 7 task evidence
  records and degraded sequential dispatch evidence for waves 1 and 2.
- `spec-compliance-review.yaml`: pass, run_version=2, references
  `layer:R0=pass`, `layer:R3=pass`, `scope_contract:pass`,
  `negative_path:pass`, and `decision_fidelity:pass`.
- `code-quality-review.yaml`: pass, run_version=2, references
  `layer:IR1=pass` and `layer:IR3=pass`.
- `goal-verification.yaml`: pass, run_version=2, references `ac:all=pass`,
  `scope_contract:pass`,
  `fresh:command_ref=verification/semgrep-goal-verification.sarif`,
  `high_risk_check:external_api_contracts.safety_baseline=pass`,
  `test:go-test-count1-all=pass`, and `coverage:change-surface=pass`.

Fresh command evidence:

- `gofmt -l` over changed Go files: no output.
- `git diff --check`: no output.
- `go build ./...`: exit 0.
- `go vet ./...`: exit 0.
- `go test ./... -count=1`: exit 0, 25 packages `ok`, 2 packages with no test
  files.
- Semgrep SAST:
  `semgrep scan --metrics=off --severity WARNING --severity ERROR --config p/golang --sarif --output artifacts/changes/add-engine-enforced-fail-closed-safety-nets-for-shared-workt/verification/semgrep-goal-verification.sarif cmd internal`
  scanned 165 Go targets with 42 Go rules and reported 0 findings; SARIF has
  1 run and 0 results.
- Coverage diagnostic:
  `go test -coverprofile=artifacts/changes/add-engine-enforced-fail-closed-safety-nets-for-shared-workt/verification/coverage-goal-verification.out ./cmd ./internal/engine/wave ./internal/engine/progression ./internal/model ./internal/state ./internal/tmpl -count=1`
  passed for affected packages; `go tool cover -func` reports total statements
  74.2%, with key new gate/advisory functions covered.

`go run . validate --json` in S4 reports fresh evidence, valid
requirements/tasks/decision contracts, `scope_contract.status=pass`, and a
passing `goal-verification` record. The remaining blocker before ship approval
is the expected missing `final-closeout` record plus its required assurance
attestation; final-closeout must record `closeout:assurance_complete=pass`.

## Evidence Index

- `verification/intake-clarification.yaml`
- `verification/research-orchestration.yaml`
- `verification/plan-audit.yaml`
- `verification/wave-orchestration.yaml`
- `verification/execution-summary.yaml`
- `verification/spec-compliance-review.yaml`
- `verification/code-quality-review.yaml`
- `verification/goal-verification.yaml`
- `verification/semgrep-goal-verification.sarif`
- `verification/coverage-goal-verification.out`
- `verification/coverage-goal-verification.func.txt`
- `verification/spec-compliance-review-notes.md`
- `verification/code-quality-review-notes.md`
- `verification/goal-verification-notes.md`
- `verification/final-closeout-notes.md` (written during final closeout)

## Requirement Coverage

- REQ-001: dispatch token alignment is implemented in
  `internal/model/wave_execution.go` and covered by model/state tests.
- REQ-002: scope-escape gating uses planned target files and
  `wave.TargetCoversPath`; tests cover out-of-scope, directory/glob coverage,
  all-within-target evidence, orphan tasks, and empty plan targets failing
  closed.
- REQ-003: parallel overlap gating is parallel-wave-only and buckets by
  `wave.CanonicalConflictPath`; tests cover overlap, sequential sharing, and
  distinct-file accept paths.
- REQ-004: dispatch evidence no longer infers parallel mode; tests cover missing
  started parallel dispatch, valid `parallel_subagents`, and
  `degraded_sequential`.
- REQ-005: executor-agent handles are required only for `parallel_subagents`;
  tests cover missing handles, all handles, degraded/non-parallel skip, and
  conflicting handles collapsing to fail-closed empty handles.
- REQ-006: wave-narrowing advisories are view-only and excluded from persisted
  wave plans and hashes; analyzer and CLI view tests cover broad targets,
  dependency-only serial plans, equivalent raw source target restoration, and
  non-equivalent source target rejection.
- REQ-007: generated host surfaces name all four blocker codes, the
  target/changed-file safety model, and the no-engine-spawn boundary; rendered
  template/reference tests enforce the content.

## Residual Risks and Exceptions

Known limits are documented in `decision.md` and accepted:

- changed-file audits are post-result checks in a shared worktree, not
  pre-write isolation;
- removing silent dispatch inference is an intentional breaking change for
  started parallel waves without dispatch evidence;
- executor-agent handles are evidence completeness checks, not OS-level proof of
  real concurrency.

No dependency, schema migration, credential, irreversible operation, or external
service call is introduced by the implementation.

## Rollback Readiness

Because this is a Slipway self-change with no migration or generated binary
artifact committed outside the source/template surfaces, rollback is a normal
git revert of the implementing commit(s). Reverting removes the new blocker
codes, dispatch literal change, view-only advisories, and generated-surface
content together.

## Archive Decision

The change is ready for final ship-gate consideration once final-closeout is
recorded for run_summary_version=2 and a fresh `go run . validate --json`
immediately before `slipway done` reports `evidence_freshness=fresh`,
`G_ship=approved`, and no blockers. Do not archive or run `slipway done` from
stale or archived-bundle evidence; the active worktree validation output is the
authority.
