# Assurance

## Scope Summary
This change repairs the documentation audit findings for multilingual Slipway
docs and generated command-surface metadata. The delivered scope includes:

- command surface parity for `slipway hook`, including root help grouping,
  generated registry metadata, command docs, and `docs/SURFACE-MANIFEST.json`;
- stale command/doc references such as `slipway config --json` and the retired
  MkDocs Material install-stack wording;
- README locale parity for Simplified Chinese and Japanese, including the
  handoff command examples and duplicate language-switcher cleanup;
- localized SVG link fixes for nested Chinese and Japanese docs;
- terminology cleanup across English, Simplified Chinese, and Japanese docs for
  the audit-listed terms and literal translations;
- root README docs badge branding, replacing the retired MkDocs badge logo with
  the current Astro branding;
- S3 required-action handling for review-absorbed task-plan drift, so final
  reviewer evidence remains authoritative while unrelated execution-summary
  blockers still fail closed;
- S3 review-authority ship-gate handling for the same review-absorbed task-plan
  drift, so terminal ship verification can consume fresh reviewer evidence
  without reopening already-reviewed task evidence.

Product behavior outside public command-surface metadata was intentionally left
out of scope.

## Verification Verdict
Implementation evidence is passing for the current execution summary. All ten
planned tasks have passing runtime task evidence after the S3 in-place `t-08`,
`t-09`, and `t-10` task-plan amendments, and `wave-orchestration` has a passing
verification record covering all S3 amendments.

The current S3 state still requires the selected review set to re-certify the
current `tasks.md`, execution summary, and task-plan hash, followed by terminal
ship-verification before the change is done-ready. This assurance record is the
deferred S3 closeout artifact required before ship verification; it is not a
claim that `done` has already been run.

## Evidence Index
- `verification/intake-clarification.yaml`: intake clarification evidence.
- `verification/plan-audit.yaml`: plan audit evidence.
- `execution/t-01-result.json`: command surface and manifest task result.
- `execution/t-02-result.json`: README parity and English install drift task
  result.
- `execution/t-03-result.json`: scoped Simplified Chinese terminology/link task
  result.
- `execution/t-04-result.json`: scoped Japanese terminology/link task result.
- `execution/t-05-result.json`: remaining Simplified Chinese docs sweep result.
- `execution/t-06-result.json`: remaining Japanese docs sweep result.
- `execution/t-07-result.json`: English wording polish result.
- `execution/t-08-result.json`: root README docs badge branding result.
- `execution/t-09-result.json`: S3 required-action handling regression result.
- `execution/t-10-result.json`: S3 review-authority ship-gate handling result.
- `verification/wave-orchestration.yaml`: recorded dispatch-mode and executor
  handles for the three parallel execution waves plus the S3 `t-08`, `t-09`,
  and `t-10` task evidence references.
- `verification/wave-orchestration-notes.md`: merged-state verification notes and
  targeted scan rationale.
- Runtime task evidence under
  `.git/slipway/runtime/changes/repair-multilingual-documentation-audit-findings-terminology/evidence/tasks/`
  for `t-01` through `t-10`.

Commands captured during execution:

- `go test ./internal/toolgen/...`
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
- `go test ./cmd -run 'TestRootHelp(UsesCurrentEntrySurfaceDescriptions|GroupsUseRegistryDescriptions)' -count=1`
- `go test ./internal/engine/governance -run 'TestResolveRuntimeRequiredActions(AbsorbsS3TaskPlanDriftAfterReviewEvidence|DoesNotAbsorbStaleExecutionEvidence|RejectsExecutionSummaryLevelBlockers)' -count=1`
- `go test ./cmd -run 'TestExecutionEvidenceBlockersStayConsistentAcrossStatusValidateAndNext' -count=1`
- `go test ./internal/engine/governance -count=1`
- `go test ./internal/engine/progression -run 'TestReviewAuthorityAbsorbsS3TaskPlanDriftOnly|TestReviewAuthorityDocsProfileIgnoresUnselectedCodeQualityEvidenceOnDisk|TestBuildShipAuthorityAttestationPresetGating' -count=1`
- `git diff --check`
- targeted terminology and stale-reference `rg` scans recorded in the task and
  wave notes.

## Requirement Coverage
- REQ-001: Covered by `t-02`, `t-03`, `t-04`, and `t-08`. Evidence includes
  removal of stale MkDocs Material install wording, locale README parity fixes,
  corrected nested SVG links under `docs/zh/` and `docs/ja/`, and root README
  docs badge branding updated to Astro.
- REQ-002: Covered by `t-01`. Evidence includes registry/root-help updates,
  generated manifest regeneration/checks, and command references that include
  `slipway hook` without exporting `$slipway-hook`.
- REQ-003: Covered by `t-03`, `t-04`, `t-05`, `t-06`, and `t-07`. Evidence
  includes full targeted scans for Chinese, Japanese, and English audit-listed
  terminology. Remaining Chinese hits are command examples, GitHub release URLs,
  or the README release badge, not prose drift.
- REQ-004: Covered by all tasks and the final `wave-orchestration` record.
  Evidence includes the toolgen test suite, manifest check, root help focused
  test, governance regression tests, progression ship-gate regression tests,
  whitespace check, targeted stale-reference scans, and Scope Contract `pass`
  after the `t-10` task evidence import.

## Residual Risks and Exceptions
- Chinese scans intentionally retain `release` in a badge/URL context and GitHub
  release download URLs, because translating those strings would break links or
  badge semantics.
- Chinese scans intentionally retain `git diff` command examples because they
  are executable Git commands, not untranslated prose.
- The documentation build and terminal ship-verification must be rerun after the
  S3 `t-10` amendment so final readiness is certified against the current
  worktree.
- Root checkout still contains pre-existing duplicate local modifications and
  scratch files from before this governed worktree was created. The active
  repair was completed in the bound worktree and does not clean unrelated root
  dirt.

## Rollback Readiness
Rollback is a normal Git revert of the modified documentation, manifest, and
toolgen metadata files. No schema migration, irreversible operation, external
service mutation, credentials, or persisted runtime data change is part of this
repair. The generated manifest can be regenerated from the Go sources with
`go run ./internal/toolgen/cmd/gen-surface-manifest --write` if a revert needs to
re-align docs metadata.

## Archive Decision
Archive readiness is conditional on the remaining S3 review set and terminal
ship-verification passing. Before `done`, the operator must capture an active
`go run . validate --json --change repair-multilingual-documentation-audit-findings-terminology`
readiness proof showing no blocking S3 gates remain. Archived bundles must be
treated as frozen project records, not as inputs revalidated through the active
validate gate after `done`.
