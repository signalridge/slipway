# Assurance

## Scope Summary
This change implements opt.md section 1.2 for the governed change
`fail-closed-missing-change`: an explicit missing target passed to
`validate --change <slug>` now fails closed with a typed `change_not_found`
precondition error instead of being softened into the unscoped no-active
diagnostic view.

The delivered scope is intentionally narrow:

- `cmd/common.go` updates the shared explicit-change resolver so a true missing
  explicit slug returns `change_not_found` with exit code 3 and `slipway status`
  remediation.
- `cmd/common.go` preserves archived explicit slug handling before the missing
  slug branch, so archived targets still fail closed as
  `archived_change_not_validatable`.
- `cmd/validate.go` is not changed; it continues to enter diagnostics fallback
  only for `no_active_change` and `active_context_ambiguous`.
- `cmd/validate_readonly_test.go`, `cmd/common_test.go`, and
  `cmd/resolve_explicit_change_authority_test.go` pin the command-level and
  resolver-level behavior for the missing explicit slug, archived explicit slug,
  unscoped no-active diagnostics, and adjacent corrupted-authority paths.

The current governed run is `run_summary_version: 1`. `tasks.md` marks t-01 and
t-02 complete, and `verification/execution-summary.yaml` records
`overall_verdict: pass` for both tasks.

## Verification Verdict
Verdict: pass for REQ-001 at `run_summary_version: 1`, subject to the terminal
`ship-verification` stamp and the final active validation recheck before
`done`.

Fresh active validation proof before `done` has been captured in S3 review with:

```text
go run . validate --json
```

After the selected peer reviews were stamped, that active worktree validation
exited 0 and reported:

- `evidence_freshness: fresh`
- `scope_contract.status: pass`
- selected S3 peer reviews ready/pass for `spec-compliance-review`,
  `code-quality-review`, and `independent-review`
- valid requirements, tasks, and decision contracts
- `G_plan: approved`
- `G_scope: approved`
- `G_ship: blocked` only because this assurance artifact and terminal
  `ship-verification` evidence/attestations had not yet been recorded

The current selected S3 peer review set has passed at `run_version: 1`:

- `spec-compliance-review.yaml`: `verdict: pass`, `blockers: []`,
  `layer:R0=pass`, `scope_contract:pass`, `negative_path:pass`, and
  `context_origin:stage=review=019f0382-257e-7b83-b5bb-87d2d7a0df66`
- `code-quality-review.yaml`: `verdict: pass`, `blockers: []`,
  `layer:IR1=pass`, and
  `context_origin:stage=review=019f0382-6a57-7f82-b697-06d04c2d6340`
- `independent-review.yaml`: `verdict: pass`, `blockers: []`, and
  `context_origin:stage=review=019f0382-9d57-7560-8bff-bcde2f7e8365`

The current implementation proof includes:

- `verification/logs/t-02-green.txt`: targeted resolver and validate command
  tests passed before S3.
- `verification/logs/t-02-cli-repro.txt`: direct
  `go run . validate --change definitely-not-a-change --json` proof returned
  JSON with `error_code: change_not_found`, `exit_code: 3`, and
  `slug: definitely-not-a-change`.
- `verification/logs/post-wave-go-test-cmd.txt`: `go test ./cmd -count=1`
  passed after implementation.
- `verification/logs/t-02-green-after-review.txt`: targeted resolver and
  validate command tests passed after the S3 stale-comment repair.
- `verification/logs/post-review-go-test-cmd.txt`: `go test ./cmd -count=1`
  passed after the S3 stale-comment repair.

No guardrail domain is set for this change, so no high-risk SAST baseline token
is required.

## Evidence Index
- `verification/execution-summary.yaml` records `run_summary_version: 1`,
  `overall_verdict: pass`, and completed tasks t-01 and t-02.
- `verification/wave-orchestration.yaml` records passing S2 wave evidence at
  `run_version: 1`.
- `verification/wave-orchestration-notes.md` records the RED/GREEN flow, the
  root cause, the live CLI proof, and the S3 comment repair with post-repair
  test transcripts.
- `verification/task-result-t-01.json` records the RED black-box validate test
  for the missing explicit slug path.
- `verification/task-result-t-02.json` records the GREEN resolver and live CLI
  proof for the `change_not_found` contract.
- `verification/spec-compliance-review.yaml` records the current passing spec
  review at `run_version: 1`.
- `verification/code-quality-review.yaml` records the current passing code
  quality review at `run_version: 1`.
- `verification/independent-review.yaml` records the current passing
  independent review at `run_version: 1`.
- `verification/logs/t-01-red.txt` captures the pre-fix regression failure:
  validate still returned success diagnostics for the explicit missing slug.
- `verification/logs/t-02-green.txt` and
  `verification/logs/t-02-green-after-review.txt` capture targeted passing
  tests for the final resolver and validate behavior.
- `verification/logs/t-02-cli-repro.txt` captures the direct missing-slug CLI
  proof with `change_not_found`.
- `verification/logs/post-wave-go-test-cmd.txt` and
  `verification/logs/post-review-go-test-cmd.txt` capture package-level
  `go test ./cmd -count=1` proof.

## Requirement Coverage
- REQ-001 missing explicit validate target is covered by
  `cmd/common.go`, `cmd/validate.go`, `cmd/validate_readonly_test.go`, and
  `cmd/resolve_explicit_change_authority_test.go`. The command path now returns
  a typed `change_not_found` precondition error with exit code 3, slug
  preservation, and `slipway status` remediation.
- REQ-001 archived explicit validate preservation is covered by
  `cmd/common.go`, `cmd/common_test.go`, and
  `cmd/validate_readonly_test.go`. Archived explicit targets remain
  `archived_change_not_validatable` and remain zero-write.
- REQ-001 unscoped no-active validation preservation is covered by
  `cmd/validate.go` and `cmd/validate_readonly_test.go`. Unscoped
  `validate --json` without an active governed change still emits the existing
  diagnostics view and remains zero-write.
- The selected resolver-level decision is covered by `cmd/common.go` and the
  resolver tests. No validate-only missing-slug branch was added, and no public
  flags, artifact schemas, runtime layout, or persisted data formats changed.
- The S3 stale-comment repair is covered by final spec, code-quality, and
  independent review notes. The comment beside the changed branch now states the
  current contract instead of the old `no_active_change` softening behavior.

## Residual Risks and Exceptions
- The broader opt.md InvocationRoute model, freshness split, S3 action contract,
  host capability model, and supply-chain items are intentionally out of scope
  for this change and remain for later governed slices.
- The changed behavior is an intentional fail-closed correction: callers that
  previously relied on explicit missing slugs being softened into
  `no_active_change` now receive `change_not_found`. That is the required public
  contract for this slice.
- No schema migrations, credential handling changes, external API contract
  changes, generated binary assets, or irreversible data operations are part of
  this change.

## Rollback Readiness
Rollback is a straightforward source and artifact rollback: revert the modified
Go files and discard the governed artifact bundle for
`fail-closed-missing-change` if the change is abandoned before archival. There
are no database migrations, generated binary artifacts, credential changes,
external service contracts, or irreversible runtime operations. If the S3
stale-comment repair were rejected independently, rollback would be limited to
the comment text in `cmd/common.go`; the behavioral resolver fix and tests are
separable.

## Archive Decision
Archive readiness decision: ready to proceed to terminal `ship-verification`,
then to the governed ship-gate recheck for the REQ-001 scope.

This assurance does not bypass the engine. Active worktree proof has been
captured before `done` with `go run . validate --json`; the current proof shows
fresh evidence, passing scope contract, and all selected S3 peer reviews
passing, with the only remaining blockers being this assurance contract and
terminal `ship-verification` evidence/attestations. `ship-verification` must
record the authoritative full-suite proof, evidence-freshness proof,
reviewer-independence attestation, and assurance-completeness attestation. After
the terminal stamp, the coordinator must run active validation again and advance
only if the current worktree reports the ship gate as approved or done-ready.
Archived bundles are frozen records and are not used as active validation input.
