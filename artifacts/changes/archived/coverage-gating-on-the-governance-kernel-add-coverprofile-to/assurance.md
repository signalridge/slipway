# Assurance

## Scope Summary

This change adds a self-contained no-regression coverage ratchet for the
governance kernel packages:

- `internal/engine/gate`
- `internal/engine/governance`
- `internal/engine/progression`

The implementation adds `internal/coverage` and
`internal/coverage/cmd/covergate`, a committed `coverage-baseline.json`, a
ubuntu-only `Kernel Coverage Gate` CI job, local `just coverage-gate` and
`just coverage-baseline` recipes, and contributor documentation. The checker
parses Go coverage profiles with union semantics for duplicate `-coverpkg`
blocks, compares each kernel package to its committed floor, and fails closed on
regression or missing coverage data. Mutation testing, external coverage
services, repo-wide gating, and coverage backfill remain out of scope.

## Verification Verdict

PASS for current run version 2.

Fresh verification evidence has been captured after the stale planning recovery:

- `go build ./...` passed.
- `go test ./internal/coverage/... -count=1` passed.
- `go vet ./internal/coverage/...` passed.
- `actionlint .github/workflows/ci.yml` passed.
- `gofmt -l internal/coverage/coverage.go internal/coverage/coverage_test.go internal/coverage/cmd/covergate/main.go internal/coverage/cmd/covergate/main_test.go` produced no output.
- `just coverage-gate` passed after running the full-suite kernel coverage path; `covergate -check` reported `gate 96.1%`, `governance 90.1%`, and `progression 81.8%` at the committed floors.

The governed evidence stack is fresh for run version 2:

- `wave-orchestration`: pass.
- `spec-compliance-review`: pass with `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`, and
  `decision_fidelity:pass`.
- `code-quality-review`: pass with `layer:IR1=pass`.
- `goal-verification`: pass with fresh command refs and `scope_contract:pass`.

## Evidence Index

- Runtime execution summary:
  `artifacts/changes/coverage-gating-on-the-governance-kernel-add-coverprofile-to/verification/execution-summary.yaml`
  (`run_summary_version: 2`, `overall_verdict: pass`).
- Wave orchestration evidence:
  `artifacts/changes/coverage-gating-on-the-governance-kernel-add-coverprofile-to/verification/wave-orchestration.yaml`.
- Spec compliance review:
  `artifacts/changes/coverage-gating-on-the-governance-kernel-add-coverprofile-to/verification/spec-compliance-review.yaml`.
- Code quality review:
  `artifacts/changes/coverage-gating-on-the-governance-kernel-add-coverprofile-to/verification/code-quality-review.yaml`.
- Goal verification:
  `artifacts/changes/coverage-gating-on-the-governance-kernel-add-coverprofile-to/verification/goal-verification.yaml`.
- Active readiness/freshness:
  `go run . validate --json` reports `evidence_freshness: fresh` and
  `scope_contract.status: pass`.

## Requirement Coverage

- REQ-001 (CI emits kernel coverage): the new CI job and `just coverage-gate`
  run full-suite `go test ./...` with `-coverpkg` scoped to the three kernel
  packages and write `tmp/coverage-kernel.out`.
- REQ-002 (fail closed on regression / pass at-or-above baseline):
  `Baseline.Check`, `covergate -check`, focused tests, and live
  `just coverage-gate` evidence cover pass-at-floor, below-floor regression,
  and missing coverage data.
- REQ-003 (union attribution): `ParseProfile` deduplicates repeated block
  locators and `TestParseProfileUnionDedup` verifies duplicate `-coverpkg`
  blocks count once.
- REQ-004 (baseline integrity / no bypass): `covergate` requires exactly one of
  `-check` or `-write`, rejects `-exclude` in check mode, validates required
  kernel floors, and has no environment skip path.
- REQ-005 (explicit exclusion list): exclusions are write-time only, are
  represented in the baseline, and cannot remove required kernel packages.
- REQ-006 (documented gate and ratchet workflow): `docs/contributing.md`
  documents the package set, local commands, union semantics, fail-closed
  behavior, exclusions, baseline ratchet, and branch-protection follow-up.

## Residual Risks and Exceptions

- The committed floors are exact current measurements. This is intentional for
  the no-regression ratchet. If a future Go toolchain or test behavior changes a
  percentage, maintainers must regenerate the baseline with `covergate -write`
  and review that diff.
- The gate only protects the three governance-kernel packages. Broader package
  coverage remains out of scope for this change.
- GitHub branch protection must be updated after the `Kernel Coverage Gate` job
  is green on `main`; the repository documentation calls out this maintainer
  follow-up.

## Rollback Readiness

The change is additive and does not alter product runtime behavior. Rollback can
remove the `coverage` CI job, delete `internal/coverage/`,
`coverage-baseline.json`, and the two justfile recipes, and remove the
contributing-doc section. After rollback, verify with `go build ./...` and the
normal CI suite. No migration, credential, external API, or irreversible data
operation is involved.

## Archive Decision

Archive after final-closeout records the required
`closeout:assurance_complete=pass` attestation and `slipway run` reports
done-ready. The active worktree has fresh run version 2 evidence, passing review
records, passing goal verification, and a passing scope contract. `slipway done`
should then archive this bundle under `artifacts/changes/archived/` while leaving
the implementation diff in the worktree for commit and PR publication.
