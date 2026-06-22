# Assurance

## Scope Summary

This change delivers issue #297 Workstream A for Slipway's command surface. It
removes `checkpoint`, `learn`, and standalone `stats` as product and
agent-facing command surfaces, and removes the checkpoint-specific lifecycle
protocol that depended on `ActiveCheckpoint`, `--resume-response`,
`resume_checkpoint`, `checkpoint_type`, checkpoint reason codes, and
checkpoint-specific status/next/run/health/repair handling.

The retained governance paths remain in scope and were preserved: `run
--resume`, interrupted-wave recovery, blocked and incomplete task verdict
blockers, `status --stats`, `health`, `evidence task`, `evidence skill`,
`evidence suite-result`, freshness checks, scope checks, overlap checks, and
fail-closed readiness behavior.

Workstreams B and C from issue #297 remain out of scope except for direct
references that had to be updated so the deleted Workstream A surfaces are no
longer advertised.

## Verification Verdict

Implementation verification is passing for the Workstream A scope. The S2
verification artifact records passing targeted Go packages, passing surface
manifest check, passing black-box help and deleted-command checks, passing
filtered live-surface searches, and a passing `go test ./...` full suite.

Current S3 state is not ship-ready yet because the selected S3 review evidence
and final-closeout evidence have not all been recorded. Fresh active validation
at S3 reports `evidence_freshness: fresh`, `scope_contract.status: pass`, and
`run_summary_version: 1`, with blockers limited to the expected missing S3
review, final-closeout, and assurance controls. This assurance file satisfies
the previously missing assurance artifact input; final readiness still depends
on the selected S3 reviewers and final-closeout passing through the Slipway CLI.

## Evidence Index

- `artifacts/changes/simplify-command-surface-a/intent.md`: approved Workstream
  A scope, exclusions, constraints, and acceptance signals.
- `artifacts/changes/simplify-command-surface-a/requirements.md`: five
  requirements covering checkpoint deletion, learn/stats deletion, generated
  surface alignment, resume/evidence invariants, and verification coverage.
- `artifacts/changes/simplify-command-surface-a/decision.md`: selected direct
  Workstream A deletion approach and rollback plan.
- `artifacts/changes/simplify-command-surface-a/tasks.md`: completed task plan
  for `t-01` through `t-06`.
- `artifacts/changes/simplify-command-surface-a/verification/implementation-verification.md`:
  implementation verification transcript summary.
- `artifacts/changes/simplify-command-surface-a/verification/wave-orchestration.yaml`:
  CLI-stamped S2 wave-orchestration pass evidence for run summary version 1.
- `artifacts/changes/simplify-command-surface-a/verification/suite-result.yaml`:
  CLI-stamped S3 suite-result keystone for run summary version 1, digesting the
  full-suite proof artifact.
- Runtime task evidence under
  `.git/slipway/runtime/changes/simplify-command-surface-a/evidence/tasks`:
  pass evidence for `t-01` through `t-06`.
- Fresh active status, validate, and next outputs captured at S3 on
  2026-06-22: `S3_REVIEW`, `run_summary_version: 1`,
  `evidence_freshness: fresh`, `scope_contract.status: pass`, selected reviews
  `spec-compliance-review`, `code-quality-review`, `independent-review`,
  `goal-verification`, and `security-review`.

## Requirement Coverage

- REQ-001 Checkpoint Surface Deletion: covered by `t-01`, `t-02`, `t-04`,
  `t-05`, and `t-06`. Evidence includes help checks proving
  `--resume-response` is absent, deleted command dispatch checks, source and
  template searches for checkpoint lifecycle tokens, and updated model/state
  tests.
- REQ-002 Learn And Stats Command Deletion: covered by `t-03`, `t-05`, and
  `t-06`. Evidence includes root-help checks proving standalone `learn` and
  `stats` are absent, unknown-command checks for the deleted commands, and a
  passing `status --stats --json` check for retained diagnostics.
- REQ-003 Generated Surface Alignment: covered by `t-05` and `t-06`. Evidence
  includes refreshed docs, generated skill inventory expectations, install
  profile metadata, `docs/SURFACE-MANIFEST.json`, and a passing
  `go run ./internal/toolgen/cmd/gen-surface-manifest --check`.
- REQ-004 Resume And Evidence Invariants: covered by `t-01`, `t-02`, `t-04`,
  and `t-06`. Evidence includes retained `run --resume`, focused progression
  and wave-sync tests, blocked/incomplete task evidence behavior, and preserved
  freshness/scope/overlap fail-closed tests.
- REQ-005 Verification Coverage: covered by `t-04`, `t-05`, and `t-06`.
  Evidence includes targeted package tests, toolgen manifest check, black-box
  command checks, filtered live-surface search checks, and `go test ./...`.

## Residual Risks and Exceptions

- Active changes that still depend on persisted checkpoint state are
  intentionally not kept compatible by this Workstream A deletion. The selected
  decision treats that as an accepted scope consequence, bounded by explicit
  fail-closed behavior instead of silently reviving checkpoint support.
- Historical references under archived change bundles and codebase-map notes are
  not treated as live product surfaces. The implementation verification filters
  those paths out when proving current product/template/docs surface deletion.
- README text outside the approved task target set may still contain older
  explanatory wording. S3 review must decide whether any such text is a live
  product surface blocker for this Workstream A scope.
- Workstreams B and C remain intentionally deferred.

## Rollback Readiness

Rollback is branch-local. The documented rollback is to revert this branch's
code, docs, template, manifest, task, and verification artifact changes
together, then rerun:

- `go test ./cmd ./internal/model ./internal/state ./internal/engine/wave ./internal/engine/progression ./internal/tmpl ./internal/toolgen`
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
- `go test ./...`

Because `ActiveCheckpoint` and checkpoint protocol state are intentionally
removed, rollback must restore model, command, state, generated-surface, and
test changes as one unit if checkpoint compatibility is needed again.

## Archive Decision

Archive readiness is pending, not granted by this assurance alone. The active
change has fresh S3 validation evidence and a passing scope contract, but
`G_ship` remains blocked until all selected S3 reviews and final-closeout are
recorded through `slipway evidence skill` and accepted by `slipway validate`.

Before `slipway done`, final-closeout must rerun active `go run . validate
--json` in the governed worktree and record the active freshness/readiness proof.
Archived bundles are frozen records only; they are not revalidated through the
active validate gate after archival.
