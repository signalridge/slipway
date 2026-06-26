# Assurance

## Scope Summary

This change repairs the public lifecycle contract for `opt.md` section 1.1
through 1.5 as one cohesive code change. The delivered scope adds a shared
invocation route/actionability projection, stable explicit `--change` error
taxonomy, readiness-safe freshness fields, shared current-action projection, and
host capability visibility with fail-closed behavior for selected delegated
actions.

The implementation changes command and model surfaces only:
`cmd/common.go`, `cmd/freshness_diagnostics.go`, `cmd/status.go`,
`cmd/status_view_build.go`, `cmd/validate.go`, `cmd/next.go`,
`cmd/next_handoff.go`, `internal/engine/capability/resolver.go`,
`internal/model/reason_code.go`, and `internal/model/recovery.go`, with focused
regression coverage in the planned command/model/capability test files. The
codebase-map edits under `artifacts/codebase/` are context refreshes and are
reported as scope-contract exempt context files.

## Verification Verdict

Current verdict: pass for implementation and selected S3 peer review, pending
the terminal `ship-verification` evidence record. `go test ./cmd -count=1`,
`go test ./... -count=1`, and `golangci-lint run ./...` passed after the S3
repair batch. The active `validate --json` run after S3 review evidence reported
`scope_contract.status=pass`, fresh execution evidence, passing selected peer
reviews, and only the expected pre-ship blockers for this assurance file and
`ship-verification`.

Selected S3 peer evidence is recorded and passing for:

- `spec-compliance-review`
- `code-quality-review`
- `independent-review`
- `security-review`

The S3 repair batch was performed by fresh-context repair subagent
`019f0400-0b39-75c0-9597-791072c7eadf`. The final selected review evidence was
recorded from distinct fresh reviewer handles:

- spec compliance: `019f0418-9ea0-7731-a3af-921230085f28`
- code quality: `019f0418-a641-7fe3-a576-d1cde9f91dcb`
- independent review: `019f0418-ac2c-77d0-aa2f-0afa5a721d29`
- security review: `019f0418-b202-7091-aa8e-290c7b29ada8`

## Evidence Index

- `verification/execution-summary.yaml`: run version 1, overall verdict pass,
  four task waves passed.
- `verification/wave-orchestration.yaml`: wave orchestration pass with command
  evidence for `go test ./cmd -count=1`, `go test ./internal/engine/capability
  -count=1`, `go test ./internal/model -count=1`, `go test ./... -count=1`,
  and `golangci-lint run ./...`.
- `verification/spec-compliance-review.yaml`: pass with `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`, and
  `decision_fidelity:pass`.
- `verification/code-quality-review.yaml`: pass with `layer:IR1=pass` and
  quality references for route matrix, host capability fallback behavior, and
  inspect-only done/archived route remediation.
- `verification/independent-review.yaml`: pass with distinct fresh review
  context and repair context references.
- `verification/security-review.yaml`: pass with distinct fresh review context
  and repair context references.
- Active `validate --json` after selected review evidence: `skills_ready`
  reports pass for spec, code-quality, independent, and security reviews;
  `scope_contract.status=pass`; `execution_evidence_freshness=fresh`;
  remaining blockers are `assurance_contract_missing` and ship-verification
  requirements, which this closeout sequence is addressing.

## Requirement Coverage

- REQ-001 is covered by the shared `invocationRouteView` and command wiring,
  plus `TestBoundWorktreeCommandsExposeConsistentLocalInvocationRoute`, root
  bound-elsewhere tests, and run/status route tests. The route matrix exercises
  `status --json`, `next --json`, and `validate --json` from the bound worktree.
- REQ-002 is covered by explicit missing-change handling returning
  `change_not_found`, archived explicit command fail-closed behavior, and
  zero-write validate tests for invalid or archived explicit changes.
- REQ-003 is covered by additive `execution_evidence_freshness`,
  `governance_evidence_freshness`, and `overall_readiness_freshness` fields,
  while preserving legacy `evidence_freshness` as execution evidence freshness.
- REQ-004 is covered by the shared current-action projection and S3 review-batch
  tests across `next`, `status`, `validate`, and `run --diagnostics`.
- REQ-005 is covered by `internal/engine/capability` resolver behavior,
  canonical `host_capability_unavailable` reason/recovery text, and command
  tests proving unknown/unavailable host capability fails closed unless explicit
  `manual_independent_review` fallback is selected.

## Residual Risks and Exceptions

- `SLIPWAY_HOST_CAPABILITIES` is an environment-declared host capability signal.
  The implementation intentionally treats missing or empty declaration as
  `unknown`, then fails closed for required independent-review subagent
  capability unless an explicit fallback is selected.
- `cmd/test_main_test.go` sets deterministic package-test defaults so existing
  review-batch tests model a capable test host. The explicit fail-closed test
  unsets and overrides that environment to prove production behavior for
  unknown, empty, unavailable, and fallback states.
- `tasks.md` was amended during S3 repair to include the test harness target.
  Current `validate --json --focus spec-trace` reports the S3 task-plan
  amendment diagnostic and keeps the change in S3 review rather than reopening
  S2, which is the intended in-place review convergence behavior.
- No compatibility exception, security bypass, or deferred product requirement
  remains for REQ-001 through REQ-005.

## Rollback Readiness

Rollback is a normal git revert of the source/test/artifact changes in this
branch before merge. There is no data migration, external API contract change,
credential format change, network integration, or persistent runtime state
change outside the governed evidence/artifact files. If rollback is needed after
merge, revert the merge commit or the feature commit and rerun the command test
suite because command JSON fields added here are public additive surfaces.

## Archive Decision

Archive readiness decision: ready to archive after terminal ship verification
passes and `slipway run` advances the change to done-ready. Active
`validate --json` proof was captured before `done` and showed the active change
was still in S3 with fresh execution evidence, passing selected peer reviews,
and only assurance/ship-verification blockers remaining. This assurance file is
the closeout artifact required before ship verification records
`closeout:assurance_complete=pass`.

Archived bundles must be treated as frozen records after `slipway done`; they
are not revalidated through the active `validate --json` gate after archive.
