# Assurance

## Scope Summary

This change resolves GitHub issue #158 by adding a generated surface inventory
manifest and fail-closed sync checks for Slipway's public generated surfaces.
The delivered scope includes:

- a deterministic manifest builder in `internal/toolgen`
- a committed public manifest at `docs/SURFACE-MANIFEST.json`
- a Go regeneration entrypoint with stdout, `--check`, and `--write` modes
- tests that fail closed when the committed manifest, docs tokens, or generated
  surface families drift
- public docs in `README.md`, `docs/ai-tools.md`, and `docs/commands.md`
- two narrow lifecycle repairs required to re-certify the governed change:
  S2 stale future-stage review evidence no longer deadlocks current-stage wave
  evidence stamping, and S2 `wave-orchestration` evidence can bootstrap from the
  current runtime task evidence ledger before `execution-summary.yaml` exists

The committed manifest is derived from existing Slipway authorities rather than
a new hand-maintained command or skill registry. Its current inventory contains
74 rows: 5 adapter rows, 21 command rows, 4 documentation rows, 21 JSON contract
rows, and 23 skill rows.

## Verification Verdict

Verdict: pass.

Execution evidence records all planned tasks `t-01` through `t-07` as passing at
`run_summary_version: 1`. Review evidence records `spec-compliance-review` and
`code-quality-review` as passing, with scope contract, negative-path coverage,
toolchain compatibility, and layer checks satisfied.

Fresh S4 proof commands after the last code/test change include:

- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
- `go test ./internal/toolgen ./internal/toolgen/cmd/gen-surface-manifest ./cmd ./internal/engine/progression -count=1`
- `go test -timeout=20m ./... -count=1`
- `go test -coverprofile=/tmp/slipway-issue158-cover.out ./internal/toolgen ./internal/toolgen/cmd/gen-surface-manifest ./cmd ./internal/engine/progression`
- `git diff --check`
- `go run . validate --json --change resolve-github-issue-158-add-a-generated-surface-inventory-m`

Before goal-verification/final-closeout are recorded, the expected remaining S4
blockers are the missing `goal-verification`, missing `final-closeout`, and the
standard-preset closeout assurance attestation.

## Evidence Index

- `verification/execution-summary.yaml`: all tasks passed with
  `run_summary_version: 1`.
- `verification/wave-orchestration.yaml`: wave execution verdict pass, recorded
  from the current task evidence ledger.
- `verification/spec-compliance-review.yaml`: verdict pass with `layer:R0=pass`,
  `scope_contract:pass`, and `negative_path:pass`.
- `verification/code-quality-review.yaml`: verdict pass with `layer:IR1=pass`
  and `toolchain_compat:pass`.
- `verification/evidence-digests.yaml`: current digests for review and
  execution evidence inputs.
- `verification/spec-compliance-review-notes.md`: forward and reverse trace for
  REQ-001 through REQ-006.
- `verification/code-quality-review-notes.md`: quality, safety, test, and
  toolchain review for the manifest and lifecycle repair diff.
- Runtime task evidence under `.git/slipway/runtime/changes/.../evidence/tasks`:
  task evidence for `t-01` through `t-07` bound to the current task-plan hash.

## Requirement Coverage

- REQ-001: Covered by `internal/toolgen/surface_manifest.go`,
  `docs/SURFACE-MANIFEST.json`, and manifest sync/docs-token tests. The builder
  derives rows from existing command, adapter, governance-skill,
  standalone/technique-skill, catalog/capability, JSON contract, and docs
  authorities, then emits deterministic sorted JSON.
- REQ-002: Covered by
  `internal/toolgen/cmd/gen-surface-manifest/main.go` and
  `internal/toolgen/cmd/gen-surface-manifest/main_test.go`. The entrypoint
  supports stdout, check, and write modes, rejects `--check --write`, and
  reports actionable stale manifest row differences.
- REQ-003: Covered by README/docs updates and tests in
  `internal/toolgen/surface_manifest_test.go`, `internal/toolgen/toolgen_test.go`,
  and `cmd/template_flag_contract_test.go`. Documentation tokens are checked and
  existing README/command contract checks remain active.
- REQ-004: Covered by governed task evidence, focused package tests, full Go
  tests, manifest check, diff check, review evidence, coverage evidence, and
  active validation.
- REQ-005: Covered by
  `internal/engine/progression/evidence_digests.go` and
  `TestStampPassingSkillDigestsDoesNotBlockCurrentStageOnFutureAcceptedEvidence`.
  The repair prevents stale future-stage review/verify evidence from blocking
  S2 current-stage re-certification before the owning future stage can refresh.
- REQ-006: Covered by `cmd/evidence.go`, `cmd/evidence_task_test.go`, and
  `docs/commands.md`. The repair lets S2 `wave-orchestration` derive its run
  version from single-version task evidence before an execution summary exists,
  while later run-summary-bound skills still fail closed without
  `execution-summary.yaml`.

## Residual Risks and Exceptions

- No unresolved implementation, review, requirement coverage, quality, or
  safety blockers are recorded.
- The populated codebase-map docs still reflect older issue #151 context and
  were treated as advisory only; they are not part of the current tracked diff.
- The manifest intentionally uses stable docs tokens instead of prose snapshots,
  so docs rewrites remain possible as long as the public surface tokens stay
  present.
- The new S2 evidence bootstrap is intentionally narrow: it applies only to
  `wave-orchestration` in S2 and rejects missing, invalid, mixed-version, or
  empty task evidence.

## Rollback Readiness

Rollback is straightforward because the change does not alter external APIs,
dependencies, lockfiles, schemas, auth, credentials, persisted user data, or
irreversible operations. Reverting the manifest builder, regeneration entrypoint,
committed manifest, docs additions, tests, and the two S2 recovery repairs
restores the previous surface.

After rollback, rerun at least:

- `go test ./internal/toolgen ./internal/toolgen/cmd/gen-surface-manifest ./cmd ./internal/engine/progression -count=1`
- `go test -timeout=20m ./... -count=1`

## Archive Decision

Archive readiness decision: ready to advance toward done-ready after
goal-verification and final-closeout are refreshed from the current worktree and
`go run . validate --json --change resolve-github-issue-158-add-a-generated-surface-inventory-m`
confirms the active ship gate is approved.

This assurance file is authored from the current issue #158 evidence record.
`slipway done` is not part of this decision and must not run without explicit
user authorization.
