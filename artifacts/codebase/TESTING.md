# Testing

Re-authored for change `resolve-current-open-issues` (#263/#170/#169/#168/#167/#161).

## Current Change Focus: Open Issue Batch

Focused coverage should land near the ownership seams rather than only as broad
end-to-end assertions:

- `cmd/evidence_skill_test.go`: reproduce #263 with a passing selected reviewer
  record that lacks a valid `context_origin:stage=review=<handle>`, prove
  replacement is allowed only for that invalid/repairable case, and prove normal
  already-current passing evidence is still rejected.
- `internal/toolgen/toolgen_test.go` or a new focused test: prove a failed
  refresh rolls back generated outputs when a later operation fails, using the
  existing `fsutil.ApplyFileTransaction` hooks/patterns instead of real crashes.
- `internal/toolgen` ownership tests: classify pristine generated, modified
  generated, and unknown user files; prove stale cleanup preserves unknown files
  and backs up or refuses modified generated files.
- `internal/toolgen` profile tests: prove profile closure includes required
  dependencies and cannot prune lifecycle-critical or sensitive-domain skills.
- Docs tests / manifest checks: update the docs surface manifest/nav contracts so
  Diataxis pages and tutorials are discoverable and command snippets remain tied
  to real command surfaces.
- `internal/testlint` analyzer tests: fixture-driven tests for source-grep and
  timing/elapsed assertions, plus explicit allowed cases for generated-surface,
  golden, and command-output contract tests.

Expected verification commands:

```bash
go test ./cmd ./internal/toolgen ./internal/testlint -count=1
go test ./... -count=1
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

Re-authored for change `generalize-digest-proof-reuse` (#258).

## Current Change Focus: Digest-Keyed Proof Reuse

The highest-value existing coverage lives in
`internal/engine/progression/authority_test.go`.
`TestCloseoutGoalVerificationReuseBlockers` already proves the current
closeout-only reuse gate accepts a valid reuse case and rejects run-version
mismatch, missing reuse run-version reference, execution evidence newer than
goal-verification, changed source content, and stale execution-summary
freshness. `TestBuildShipAuthoritySurfacesCloseoutReuseBlocker` proves the reuse
blocker is surfaced through both verification blockers and `G_ship` reason codes.

Digest coverage lives in `internal/engine/progression/evidence_digests_test.go`.
`TestGoalVerificationDigestStalesWhenSharedSuiteInputsChange` proves changing
the suite-result full-suite digest stales goal-verification rather than falling
back to an unavailable generic digest. Fixture helpers already create
`suite-result.yaml` with `run_summary_version` and `full_suite_digest`, which is
the right starting point for SAST digest and cross-stage proof-reuse tests.

The implementation should extend these focused suites before broadening to a
full repository test run:

- Generalized proof-reuse validator accepts the existing closeout -> goal path.
- Invalidations stay fail-closed for changed artifacts/source, run-version
  mismatch, missing suite-result proof, missing guardrail SAST digest, and stale
  execution-summary freshness.
- `cmd/evidence_skill_test.go` covers selected-review restamp recovery when an
  existing passing reviewer record has a malformed or retired
  `context_origin:stage=review=<handle>` token.
- Final-closeout template tests show reuse-first wording when validation applies
  and rerun fallback when validation fails.
- Goal-verification template tests must not imply unconditional S2 proof reuse.

Expected focused command before completion claims:

```bash
go test ./internal/engine/wave ./internal/engine/progression -count=1
go test ./cmd -run 'TestEvidenceSkillAllowsSelectedReviewerRestampForInvalidContextOrigin$' -count=1
```

Re-authored for change
`feat-governance-host-native-subagent-enforced-cross-stage-in` (#240).

Baseline: #239 (engine-consumed reviewer-independence + context-origin/`review_origin`
attestation, closeout chain-ordering, and the wave dispatch/executor gates) has
SHIPPED (commit 2d2adac, 0.26.0); this change builds on that baseline and partly
retires it, replacing `review_origin` with the chain-wide `context_origin` lattice.

## Existing Coverage

- `internal/engine/progression/authority_test.go` now covers the selected review
  lattice directly. The selected-review tests prove same-handle reviewers collide
  by skill name, a passing selected reviewer with no
  `context_origin:stage=review=<handle>` fails closed, unselected security evidence
  on disk stays silent, selected security participates when selected, and ship
  authority does not double-fire review-owned edges. The closeout chain-order test
  asserts goal verification must be at or after every selected review verdict and
  final closeout must not predate goal verification.
- `internal/engine/skill/skill_test.go` and
  `internal/engine/progression/skill_resolution_test.go` pin the selected review
  set. The mandatory set contains:
  - `spec-compliance-review`
  - `code-quality-review`
  - `independent-review`
  and the selected-security case appends `security-review`. Routing returns the
  selected peer set as a slice; no reviewer depends on a predecessor.
- Command and view tests (`cmd/progression_next_test.go`,
  `cmd/governance_gate_consistency_test.go`, `cmd/status_view_build_test.go`,
  `cmd/review_test.go`, `cmd/evidence_skill_test.go`, and stats tests) assert
  `selected_review_skills` is surfaced consistently, selected-security missing
  evidence is blocking when selected, and unselected security evidence is rejected
  or ignored at the correct boundary.
- `internal/tmpl/templates_test.go` and `internal/toolgen/toolgen_test.go` pin the
  promoted review host surfaces. All selected review templates emit
  `context_origin:stage=review=<handle>`, independent/security hosts export as
  workflow-owned S3 skills, and retired review-origin / review-context token forms
  stay absent.
- `internal/model/context_attestation_test.go`,
  `internal/model/reason_code_contract_test.go`, and
  `internal/model/recovery_test.go` pin the pure grammar, canonical blocker
  vocabulary, and recovery table. `StageContextReview` is frozen as the only new
  shared review wire stage; reviewer participant identity stays outside the model
  helper and is supplied by the authority feeder.

## Gaps Closed By This Change

- Variable review routing and requiredness: the S3 path now uses one selected set
  for routing, required-skill filtering, public status/next/validate surfaces, and
  stale-evidence recovery.
- Skill-keyed R2 lattice: selected reviewers all emit the shared `stage=review`
  token, but the authority map keys participants by selected skill name so duplicate
  reviewer handles fail closed instead of being deduped.
- Selected-set chain order: `closeout_chain_order_invalid` compares every selected
  reviewer before goal verification and final closeout after goal verification,
  independent of the goal-verification reuse token.
- Fail-closed recovery: missing selected reviewer evidence, malformed
  `stage=review` handles, reviewer handle collisions, and selected-set ordering
  violations surface canonical blockers that route through public skill re-runs and
  engine-stamped evidence only.
- Trust-tier clarity: tests and docs treat all host-emitted context handles as
  structural/audit evidence, not cryptographic proof of independent contexts.

## Verification Plan

- Focused: `go test ./internal/engine/progression ./internal/model ./internal/tmpl ./internal/toolgen ./cmd`.
- Full suite: `go test ./...` (29 packages).
- Layering: `go test ./internal/architecture` (dependency_direction_test must
  still forbid `internal/model` and `internal/state` importing
  `cmd`/`tmpl`/`toolgen`).
- Formatting/lint: `gofmt -s -l .` clean; `golangci-lint run` (gofmt-simplify).
- Dogfood: after evidence refreshes, current-worktree `slipway status` /
  `validate` / `next --json` to confirm the new gate surfaces and routes
  recovery through the public flow.
