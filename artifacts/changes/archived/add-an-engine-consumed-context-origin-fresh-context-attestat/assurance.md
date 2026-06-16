# Assurance

## Scope Summary
Implements Option A — the honest hybrid that makes context-origin/reviewer
independence engine-consumed rather than decorative — in four parts wired into
the authority/wave-sync seams that already own the relevant evidence records:

- P1 (authority.go ship gate): require `closeout:reviewer_independence=pass`
  present on the final-closeout record, and promote the cross-stage ordering
  halves out from behind the opt-in `closeout:goal_verification_reuse=pass`
  early-return into an always-on invariant `closeout ≥ goal ≥ max(spec, code)`
  carrying its own distinct reason code.
- P2 (review authority): require both spec-compliance-review and
  code-quality-review to record a `review_origin:` handle and require the two
  handles to differ.
- P4 (wave_sync.go): #5 — for shared `target_files`, a `task_kind=test` task must
  be structurally distinct from and dispatched before its dependent
  `task_kind=code` task, derived only from engine-owned `task_kind`+`target_files`
  (never `session_id`); #6 — a bare `degraded_sequential` dispatch is rejected
  unless paired with a tool-unavailable justification reference token, on the
  `evidence skill` path as well as advance/next.
- Supporting surfaces: a new pure `internal/model` grammar (`review_origin:` and
  degraded-justification tokens + parsers, free of cmd/tmpl/toolgen imports),
  five new canonical reason codes across the three-file reason-code/recovery
  contract, the five generated review/closeout/goal/wave templates that emit and
  explain the tokens, and docs/{workflow,design}.md.

All new gates fail closed at error severity on standard/strict
(`EffectivePreset != light`) and are advisory on light, with no bypass,
force-close, or self-stamp path.

## Verification Verdict
Implementation is complete and the full suite is green from this worktree:
`go test ./...` = 27 packages ok, 0 FAIL; `go build ./...`, `go vet ./...`, and
`gofmt -s -l` all clean (re-verified fresh this session, not inherited). S2
execution evidence (t-01..t-04 + wave-orchestration) is recorded at run-summary
version 1 with every task's `changed_files ⊆ target_files`. Final S3 independent
verdicts are carried by the spec-compliance-review and code-quality-review
records (each with a distinct `review_origin:` handle, dogfooding P2); S4 carries
goal-verification and final-closeout with the chain-order and
reviewer-independence attestations this change introduces (dogfooding P1).

## Evidence Index
- Execution: `verification/wave-plan.yaml`, `verification/wave-orchestration.yaml`,
  runtime `evidence/tasks/t-0{1..4}.json`, runtime `evidence/waves/wave-0{1..4}.yaml`
  (run-summary version 1).
- Planning: `verification/plan-audit.yaml`, `verification/research-orchestration.yaml`,
  `verification/intake-clarification.yaml`.
- Review (S3): `verification/spec-compliance-review.yaml`,
  `verification/code-quality-review.yaml` (distinct `review_origin:` handles).
- Final (S4): `verification/goal-verification.yaml`, `verification/final-closeout.yaml`.
- Test signal: `go test ./...` = 27 ok; build/vet/gofmt clean.

## Requirement Coverage
- REQ-001 (P1 reviewer-independence presence + always-on chain order, 2 distinct
  codes) → t-02 (`authority.go`, `authority_test.go`, `freshness_guard_test.go`).
- REQ-002 (P2 distinct review_origin handle pair) → t-02 (`authority.go` review
  authority) + t-01 (`context_attestation.go` grammar).
- REQ-003 (P4#5 test≠impl distinctness on task_kind+target_files) → t-03
  (`wave_sync.go`, `wave_sync_test.go`).
- REQ-004 (P4#6 degraded_sequential requires paired justification) → t-03.
- REQ-005 (three-file reason-code/recovery contract, no `unknown_reason_code`) →
  t-01 (`reason_code.go`, `reason_code_contract_test.go`, `recovery.go`,
  `recovery_test.go`).
- REQ-006 (templates + docs emit/explain tokens) → t-04 (5 SKILL templates,
  `templates_test.go`, `docs/workflow.md`, `docs/design.md`).
- REQ-007 (no bypass + honest residual + dogfood) → t-02 + t-04; this change
  satisfies its own new gates through the S3/S4 dogfood.

## Residual Risks and Exceptions
- Honest residual (P3): true non-forgeable distinct-context discrimination
  (Option B) is infeasible within this change's constraints — the host controls
  only `References`, the four independence skills share a run-summary version, and
  the only zero-schema engine nonce is host-readable. The `review_origin:` handle
  gate is therefore audit/structural tier (it raises forging cost and gives an
  auditable per-review label), never cryptographic proof. This is documented in
  decision.md, docs/design.md, and the review templates rather than hidden.
- P4#5 is structurally vacuous for this change's own bundle (four single-task
  waves with disjoint target_files), so its enforcement is covered by unit tests
  rather than this change's dogfood; this is expected and noted.
- Unrelated pre-existing friction (intent.md checkbox edit re-staling intake) was
  filed as issue #238 and is out of scope here.

## Rollback Readiness
Rollback is a clean `git revert`/branch drop: the change is additive gates plus a
new pure `internal/model` file and template/doc edits, with no schema migration,
no data format change, and no irreversible operation. The new gates are
preset-scoped (advisory on light) and fail closed to rerun/review/evidence, so
reverting restores the prior ship/review behavior without orphaned state. The
feat branch is currently based on the unmerged codex commit `c7a828d`; the PR
will rebase `--onto origin/main` to drop it, keeping the revert surface to this
change's own commits.

## Archive Decision
Not yet archived. Archive readiness will be asserted only after `slipway done`
captures fresh active `validate --json` readiness proof from this worktree at the
ship gate; no archived bundle is or will be described as revalidated through the
active validate gate. The change remains in the governed lifecycle (S3 at time of
authoring) and will be archived through the public `slipway done` flow once S3
reviews and S4 goal/closeout evidence are recorded and the ship gate is clean.
