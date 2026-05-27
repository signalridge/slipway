# Assurance
## Project Context
- Tech Stack: Go CLI
- Conventions:
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
This change removes obsolete compatibility and metadata layers while preserving
the governance fields that still carry behavioral meaning.

Delivered scope:
- Removed the `--quick` flag from `slipway next` and `slipway run`, including
  the transient `QuickMode` progression bypass that injected disabled advisory
  controls.
- Tightened task evidence parsing to the current flat JSON shape. Runtime task
  evidence now requires `task_id`, `run_summary_version`, `task_kind`,
  `verdict`, `evidence_ref`, and `captured_at`; nested `task_run` and
  best-effort defaults are rejected.
- Removed unused artifact-level version metadata from `ArtifactState` and the
  artifact template data while preserving run, schema, lifecycle, wave-plan,
  governance-snapshot, and policy-pack versions.
- Replaced durable docs comparison tables with Slipway-owned design boundaries.
- Normalized tracked archived references away from machine-local upstream paths
  and refreshed matching archived artifact hashes.

## Verification Verdict
Pass. Focused regression tests, full repository tests, build, docs, whitespace
checks, and Slipway validation were rerun after the review fixes. The current
change is done-ready once `slipway done` archives it.

## Evidence Index
- `verification/wave-orchestration.yaml`: execution summary and focused/full
  implementation evidence.
- `verification/spec-compliance-review.yaml`: requirement-to-implementation
  review for REQ-001 through REQ-006.
- `verification/code-quality-review.yaml`: line-level quality review and safety
  check.
- `verification/goal-verification.yaml`: acceptance criteria verification after
  review fixes.
- `verification/final-closeout.yaml`: final closeout proof for tests, build,
  docs, validation, assurance completeness, and staged diff hygiene.
- `verification/execution-summary.yaml`: run-version-bound task evidence for
  t-01 through t-05.

## Requirement Coverage
- REQ-001: Covered by `cmd/root_help_test.go`, removal of quick-mode command
  flags, removal of `AdvanceOptions.QuickMode`, and focused command tests.
- REQ-002: Covered by `ParseTaskEvidence` strict validation and
  `TestParseTaskEvidenceRejectsCompatibilityFallbacks`.
- REQ-003: Covered by removal of `ArtifactState.Version`,
  `ManifestVersion`, materialization defaults, model round-trip updates, and
  archived `change.yaml` cleanup.
- REQ-004: Covered by `docs/design.md`, `docs/index.md`, `docs/workflow.md`,
  root-cause tracing reference cleanup, open-question tests, and normalized
  archived research/intent references.
- REQ-005: Covered by preservation of lifecycle-log compatibility, worktree
  fallback behavior, marker-gated OpenCode cleanup, compact JSON projections,
  and intentional run/schema versions.
- REQ-006: Covered by focused regression tests, `go test ./... -count=1`,
  `go build ./...`, `mkdocs build --strict`, `git diff --check`, and
  `go run . validate --json --change ...`.

## Residual Risks and Exceptions
- Removing `--quick` can break private scripts that discovered the hidden flag.
  This is accepted because the flag was a governance bypass and not documented
  in root help.
- Tightened task evidence can reject manually authored stale evidence. This is
  accepted because the current contract requires explicit flat task evidence.
- Archived artifact text was edited only to remove machine-local references and
  obsolete examples; matching archived `content_hash` metadata was refreshed.
- Archive path-generation semantics and stricter config-policy behavior remain
  out of scope for a future change.

## Rollback Readiness
Rollback is a direct source revert of this governed change. No data migration is
required. The only persisted format effect is omission of unused per-artifact
`version: 1` metadata from future `change.yaml` writes; behaviorally meaningful
version fields remain intact.

## Archive Decision
Ready to archive after final closeout. The current worktree has passing tests,
build, docs, validation, review records, goal verification, complete assurance,
and refreshed archived artifact metadata.
