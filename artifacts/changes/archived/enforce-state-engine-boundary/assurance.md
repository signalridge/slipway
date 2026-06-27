# Assurance

## Scope Summary

This change enforces the production dependency boundary between `internal/state`
and `internal/engine` while preserving the governed lifecycle behavior that uses
state for persisted artifacts.

Delivered scope:

- `internal/freshness` now owns the engine-neutral evidence freshness status and
  structural comparison helpers. `internal/engine/context` retains only execution
  mode context.
- `internal/wave` now owns the engine-neutral task-plan parsing, hashing,
  projection, and wave planning primitives. The old `internal/engine/wave`
  package was deleted with no compatibility shim.
- `internal/state` now consumes `internal/freshness` and `internal/wave` for
  persistence-oriented execution summary and wave-plan behavior without importing
  `internal/engine` production packages.
- `internal/architecture/dependency_direction_test.go` now fails production
  `internal/state -> internal/engine` imports while continuing to ignore
  `_test.go` integration fixtures.

## Verification Verdict

Current S3 peer review verdict: pass.

The review set selected by the workflow is complete and passing:

- `spec-compliance-review`: pass, recorded in
  `verification/spec-compliance-review.yaml` with `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`, and context handle
  `019f06c5-5318-7f01-8763-763c7dd3fe63`.
- `code-quality-review`: pass, recorded in
  `verification/code-quality-review.yaml` with `layer:IR1=pass` and context
  handle `019f06c5-9bf1-7303-903e-77c1d21b59fb`.
- `independent-review`: pass, recorded in
  `verification/independent-review.yaml` with context handle
  `019f06d4-e6bc-7582-ba4e-87453c6cdd85`.
- `security-review`: pass, recorded in `verification/security-review.yaml` with
  `security_review:pass` and context handle
  `019f06dc-44bd-78d2-bd15-911fa205f928`.

Fresh active validation before terminal ship verification:

- `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate --json` reported
  `evidence_freshness=fresh`, `execution_evidence_freshness=fresh`,
  `scope_contract.status=pass`, and the four selected peer review skills passing.
- The remaining S3 blockers at that point were limited to this deferred
  `assurance.md` artifact and the always-required `ship-verification` terminal
  gate.

Terminal ship verification must run after this artifact is authored and before
`done`. It must include a fresh full-suite command transcript, fresh lint/static
check proof, stub scan proof, assurance completeness attestation, reviewer
independence attestation, and a final active validation proof before archive.

## Evidence Index

- Task evidence:
  - `t-01`: `tests:t-01-targeted-freshness`
  - `t-02`: `tests:t-02-targeted-wave`
  - `t-03`: `tests:t-03-architecture-and-full-suite`
- Implementation verification notes:
  - `verification/architecture-boundary-tests.md`
  - `verification/wave-orchestration-notes.md`
- Peer review notes and CLI-stamped evidence:
  - `verification/spec-compliance-review-notes.md`
  - `verification/spec-compliance-review.yaml`
  - `verification/code-quality-review-notes.md`
  - `verification/code-quality-review.yaml`
  - `verification/independent-review-notes.md`
  - `verification/independent-review.yaml`
  - `verification/security-review-notes.md`
  - `verification/security-review.yaml`
- Commands already recorded in verification notes:
  - `go test ./internal/architecture -count=1`
  - `rg -n 'github.com/signalridge/slipway/internal/engine|internal/engine/' internal/state`
  - `go test ./internal/freshness ./internal/engine/context ./internal/state ./internal/engine/progression ./cmd -run 'Test.*Freshness|TestEvaluateEvidenceFreshness|TestProjectExecutionFreshness|TestExecutionSummary' -count=1`
  - `go test ./internal/wave ./internal/state ./internal/engine/progression ./internal/engine/governance ./internal/engine/scopecontract ./internal/engine/artifact ./internal/engine/status ./cmd -run 'Test.*Wave|Test.*TaskPlan|Test.*Scope|Test.*Materialize|Test.*Repair|Test.*Health|Test.*Evidence' -count=1`
  - `go test ./... -count=1`
  - `golangci-lint run --timeout 5m`
  - `git diff --check HEAD`

## Requirement Coverage

REQ-001: State has no engine production dependency.

- Exists: `internal/architecture/dependency_direction_test.go` contains the
  package-specific forbidden import rule for production files under
  `internal/state`.
- Substantive: the rule matches exact `github.com/signalridge/slipway/internal/engine`
  imports and subpackage imports, and reports file-level violations.
- Wired: `go test ./internal/architecture -count=1` passed, and the live grep
  for `github.com/signalridge/slipway/internal/engine|internal/engine/` under
  `internal/state` returned no matches.

REQ-002: State remains a persistence layer.

- Exists: `internal/freshness/freshness.go` and `internal/wave/*` provide the
  lower-level packages consumed by state.
- Substantive: `internal/state/execution_summary.go` preserves public freshness
  values `fresh`, `stale`, and `unknown` through `internal/freshness`;
  `internal/state/wave_execution.go` continues strict load/save behavior around
  `model.WavePlan` while delegating parser/hash/planning primitives to
  `internal/wave`.
- Wired: targeted freshness and wave tests passed, and independent review
  confirmed `internal/freshness/freshness.go` is tracked in the diff while
  `internal/engine/wave` is absent.

REQ-003: Lifecycle behavior is preserved.

- Exists: command, progression, governance, artifact, scope contract, status,
  state, wave, freshness, and architecture packages all have changed test
  coverage or peer review evidence.
- Substantive: no compatibility package remains at `internal/engine/wave`; all
  current Go consumers were updated to `internal/wave`; freshness helper removal
  from `internal/engine/context` is covered by relocated tests and guard tests.
- Wired: targeted package tests, `go test ./... -count=1`, `golangci-lint run
  --timeout 5m`, and `git diff --check HEAD` were recorded green before terminal
  ship verification. Ship verification must rerun the authoritative full suite
  after this assurance artifact is present.

## Residual Risks and Exceptions

- Scope contract includes some planned target files that did not require edits
  after the final implementation shape was chosen. `validate --json` reports the
  scope contract as `pass`, so this is not an accepted scope exception.
- `internal/engine/context` remains as the owner of `ExecutionModeGoverned`;
  this is intentional and not a compatibility layer for removed freshness APIs.
- Test-only integration imports remain allowed by the architecture gate. This is
  intentional because REQ-001 applies to production imports.
- No external service, data migration, schema migration, dependency update, or
  security-sensitive runtime surface was introduced.

## Rollback Readiness

Rollback is a normal revert of the PR. There is no data migration, external
state mutation, artifact schema change, or runtime storage migration to unwind.

Rollback constraints:

- Reverting restores the old package layout and removes the new architecture
  gate, so it would also restore the dependency direction problem this change is
  intended to prevent.
- Because no serialized `wave-plan.yaml`, execution summary, or wave evidence
  schema was changed, rollback does not require cleanup of persisted governed
  artifacts.

## Archive Decision

Archive decision: ready after terminal ship verification passes and active
validation is green before `done`.

Rationale:

- The implementation and governed evidence are aligned with `requirements.md`,
  `decision.md`, and `tasks.md`.
- The selected S3 review peers are recorded with distinct review-stage context
  handles and passing verdicts.
- Active validation has already been run before `done`; it reported fresh
  execution/governance inputs for the completed peer-review stage and identified
  only the expected remaining `assurance.md` and `ship-verification` blockers.
- A final active `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate --json`
  proof must be captured after ship-verification evidence is recorded and before
  `go run . done --json`. Archived bundles are not treated as active validation
  surfaces after finalization.
