# Assurance

## Scope Summary
Delivered a focused state-read performance baseline for the `opt.md` 4.1
requirement. The change adds:

- `internal/perfbaseline` JSON baseline and threshold comparison logic;
- `internal/perfbaseline/cmd/state-read-baseline`, a repo-native tool that
  builds or accepts a `slipway` binary, prepares a synthetic state-read fixture,
  warms each required lifecycle command, records the fastest timed sample,
  and checks regressions across bounded independent attempts without letting
  check mode overwrite the committed baseline by default;
- `state-read-performance-baseline.json`, the initial built-binary baseline
  artifact;
- operator documentation for refresh and check commands.

No lifecycle route, freshness, action-contract, release, GitHub settings, or
persistent cache behavior was changed.

## Verification Verdict
Implementation verification passed:

- targeted package tests passed;
- baseline refresh passed and wrote `state-read-performance-baseline.json`;
- baseline check passed against the committed baseline with a 30% budget;
- full repository `go test ./... -count=1` passed.

## Evidence Index
- `artifacts/changes/add-state-read-performance-baseline/verification/implementation-tests.txt`
- `artifacts/changes/add-state-read-performance-baseline/verification/state-read-current.json`
- `artifacts/changes/add-state-read-performance-baseline/verification/full-suite.txt`
- `.git/slipway/runtime/changes/add-state-read-performance-baseline/evidence/tasks/t-01.json`
- `.git/slipway/runtime/changes/add-state-read-performance-baseline/evidence/tasks/t-02.json`
- `.git/slipway/runtime/changes/add-state-read-performance-baseline/evidence/tasks/t-03.json`
- `.git/slipway/runtime/changes/add-state-read-performance-baseline/evidence/tasks/t-04.json`
- `artifacts/changes/add-state-read-performance-baseline/verification/wave-orchestration.yaml`

## Requirement Coverage
- REQ-001: Covered by `state-read-baseline` refresh mode and
  `state-read-performance-baseline.json`, which records root status, bound
  status, bound next diagnostics, bound validate, and explicit `--change`
  status measurements.
- REQ-002: Covered by the committed baseline artifact, including
  `real_ms`/`user_ms`/`system_ms`, fixture worktree/change/verification counts,
  Go version, git commit, binary path, warmup/sample/check-attempt counts, and
  repeat commands.
- REQ-003: Covered by `perfbaseline.Compare` tests and `state-read-baseline
  -mode check`, which reports command-specific threshold regressions.
- REQ-004: Covered by targeted `go test ./internal/perfbaseline
  ./internal/perfbaseline/cmd/state-read-baseline -count=1` and full
  `go test ./... -count=1`.

## Residual Risks and Exceptions
- Local timing measurements remain host-sensitive. The tool reduces incidental
  noise with one warmup, the fastest of seven timed samples, and up to three
  independent check attempts. The 30% threshold is still a guardrail for
  interactive regressions rather than a statistically rigorous benchmark claim.
- The committed baseline includes the local temporary fixture and binary paths
  from the measurement run. They are audit metadata for that run; repeatability
  comes from the recorded refresh command and fixture generator.
- Generated fixtures are intentionally temporary by default. Use
  `-keep-fixture` only for local diagnosis.

## Rollback Readiness
Rollback is straightforward: remove `internal/perfbaseline`, remove
`state-read-performance-baseline.json`, and revert the operator-guide section.
No persisted runtime schema or lifecycle authority format changes are involved.

## Archive Decision
Archive after current-worktree review and ship-verification gates pass. Capture
active `validate --json` freshness/readiness proof before `done`; archived
bundles are frozen records and are not revalidated through the active validate
gate.
