# Assurance

## Scope Summary
This change delivers the public evidence and recovery UX fixes for GitHub issues
#310 and #311, including the #310.1 wave-plan freshness repair. The governed
scope is complete for five requirements:

- REQ-001 rejects duplicate non-empty task result `session_id` values during
  `slipway evidence task --result-file` import before invalid task evidence is
  persisted.
- REQ-002 keeps malformed engine-owned `verification/<skill>.yaml` records
  fail-closed while explaining the verification-record boundary and the
  `--notes-file` recovery path.
- REQ-003 makes ready no-skill lifecycle states report advancement guidance
  instead of governance-blocked wording.
- REQ-004 distinguishes archived same-slug active-state residue from the
  archived delivered record and avoids primary `--worktree` remediation text.
- REQ-005 excludes lifecycle bookkeeping from the `wave-orchestration`
  wave-plan freshness digest so freshly stamped evidence does not immediately
  stale itself after equivalent wave-plan rematerialization, while genuine
  task-plan structural, scope, or semantic changes still stale the evidence.

The current governed run is `run_summary_version: 2`. `tasks.md` marks t-01
through t-05 complete, and `verification/execution-summary.yaml` records
`overall_verdict: pass` for all five tasks.

## Verification Verdict
Verdict: pass for the delivered AC-1 through AC-5 scope at
`run_summary_version: 2`, subject to the engine-owned final-closeout stamp and
ship-gate recheck.

The latest active validation proof before `done` was captured in S3 review by
the independent final-closeout verifier with:

```text
go run . validate --json
```

That active worktree validation exited 0 and reported:

- `evidence_freshness: fresh`
- `scope_contract.status: pass`
- selected S3 skills ready/pass for `spec-compliance-review`,
  `code-quality-review`, `independent-review`, `goal-verification`, and
  `security-review`
- valid requirements and tasks contracts
- `G_plan: approved`
- `G_ship: blocked` only because `final-closeout` evidence and the
  final-closeout assurance and reviewer-independence attestations had not yet
  been recorded

The full-suite proof for this run is
`verification/logs/goal-verification-go-test-all-count1-rv2.txt`, which records
`go test ./... -count=1`, `started_at: 2026-06-26T09:21:01Z`,
`finished_at: 2026-06-26T09:24:24Z`, and `exit_code: 0`. The current
`verification/suite-result.yaml` binds that proof to `run_summary_version: 2`
with digest
`sha256:c7f21d24903a6f958595d3e18bf696c6703ecc087362d6304513715389587c4c`.

The selected S3 review set has passed at `run_version: 2`:

- `spec-compliance-review.yaml`: `verdict: pass`, `blockers: []`,
  `run_version: 2`
- `code-quality-review.yaml`: `verdict: pass`, `blockers: []`,
  `run_version: 2`
- `independent-review.yaml`: `verdict: pass`, `blockers: []`,
  `run_version: 2`
- `goal-verification.yaml`: `verdict: pass`, `blockers: []`,
  `run_version: 2`
- `security-review.yaml`: `verdict: pass`, `blockers: []`,
  `run_version: 2`

No guardrail domain is set for this change, so no high-risk SAST baseline token
is required.

## Evidence Index
- `verification/execution-summary.yaml` records
  `run_summary_version: 2`, `overall_verdict: pass`, and completed tasks
  t-01 through t-05.
- `verification/wave-orchestration.yaml` records the passing wave evidence for
  all five tasks at `run_version: 2`, including the repair executor handle for
  t-05.
- `verification/suite-result.yaml` records the current full-suite digest for
  `run_summary_version: 2`.
- `verification/logs/goal-verification-go-test-all-count1-rv2.txt` captures the
  current full-suite proof, `go test ./... -count=1`, exit 0.
- `verification/logs/t-05-focused.txt` captures the focused REQ-005 regression
  proof, exit 0.
- `verification/task-result-t-01.json` through
  `verification/task-result-t-05.json` record task-level proof for REQ-001
  through REQ-005.
- `verification/spec-compliance-review.yaml` records the current passing spec
  review at `run_version: 2`.
- `verification/code-quality-review.yaml` records the current passing code
  quality review at `run_version: 2`.
- `verification/independent-review.yaml` records the current passing independent
  review at `run_version: 2`.
- `verification/goal-verification.yaml` records the current passing goal
  verification at `run_version: 2`.
- `verification/security-review.yaml` records the current passing security
  review at `run_version: 2`.
- `verification/final-closeout-notes.md` records the independent final-closeout
  verifier's pre-stamp validation, proof-reuse judgment, and the assurance
  repair required before final-closeout evidence can be stamped.

## Requirement Coverage
- REQ-001 is covered by `cmd/evidence.go` and regression tests in
  `cmd/evidence_task_test.go`, including duplicate sessions in one import batch
  and duplicate sessions against existing active-run task evidence.
- REQ-002 is covered by `internal/state/verification.go`,
  `internal/state/verification_test.go`, `cmd/validate_readonly_test.go`, and
  generated `wave-orchestration` skill guidance; malformed verification YAML
  remains invalid and now carries actionable notes-file recovery guidance.
- REQ-003 is covered by `cmd/next.go` and `cmd/progression_next_test.go`;
  ready no-skill states now surface the `slipway run` command boundary instead
  of `blocked_by_governance` wording.
- REQ-004 is covered by `cmd/common.go`, `cmd/status.go`,
  `internal/model/recovery.go`, `cmd/orphaned_bundle_unmanaged_worktree_test.go`,
  and `internal/model/recovery_test.go`; archived same-slug active residue now
  targets only incomplete active-state residue and omits `--worktree` from the
  primary remediation.
- REQ-005 is covered by
  `internal/engine/progression/evidence_digests.go` and
  `internal/engine/progression/evidence_digests_test.go`; the wave-plan digest
  excludes run-summary version, per-wave parallel flag, and generated-at
  timestamp bookkeeping while preserving staleness for genuine task-plan scope
  changes.

## Residual Risks and Exceptions
- There is no remaining assurance exception for #310.1; it is covered by
  REQ-005 and t-05.
- The REQ-005 digest exclusion is intentionally limited to known lifecycle
  bookkeeping fields. Future wave-plan bookkeeping fields should be evaluated
  before they are added to the freshness digest input.
- The selected S3 reviewers and goal-verification have passed at the current
  run version. The only remaining ship-gate work is the engine-owned
  `final-closeout` evidence stamp, followed by active validation and governed
  lifecycle advancement.
- This change does not include schema migrations, credential handling changes,
  external API contract changes, generated binary assets, or irreversible data
  operations.

## Rollback Readiness
Rollback is straightforward source and artifact rollback: revert the modified Go
files and discard the governed artifact bundle for
`fix-evidence-and-recovery-ux` if the change is abandoned. There are no schema
migrations, external API contracts, credential changes, generated binary assets,
or irreversible data operations. If only the t-05 repair were rejected, rollback
would be limited to the REQ-005 digest change, its regression test, and the
associated governed task and assurance artifacts.

## Archive Decision
Archive readiness decision: ready to proceed to the governed final-closeout
stamp and ship-gate recheck for the full REQ-001 through REQ-005 scope.

This assurance does not bypass the engine. Before any `done` command, active
worktree proof has been captured with `go run . validate --json`, and the
engine must accept a passing `final-closeout` record containing the required
freshness, proof-reuse, reviewer-independence, and assurance-completeness
attestations. After the final-closeout stamp, the coordinator must run active
validation again and advance only if the current worktree reports the ship gate
as approved or done-ready. Archived bundles are frozen records and are not used
as active validation input.
