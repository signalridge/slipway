# Assurance

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Resolve #66, #70, and #67 in one critical change under one principle — never project a
governance signal to a scalar at the consume boundary. #66 + #70 rebind skill-evidence and
stale-planning-chain freshness to an engine-owned content digest
(`verification/evidence-digests.yaml`, Approach B′); `plan-audit` includes `assurance.md`;
`final-closeout` binds `assurance.md` in addition to the goal-verification changed/target
file set; legacy file-absent migration and refreshed-verdict restamps share a one-time
verdict-timestamp safety gate against the same certified digest input-set; #70 stops
deriving `wave-plan.generated_at` from `tasks.md` mtime and makes it display/audit
materialization time only while freshness keys on `tasks_plan_hash`; #67 routes S4
post-review remediation to goal-verification + final-closeout and names changed artifacts
when digest diagnostics are available. #59 traceability legibility is split into a separate
quick PR and #59 stays open with a per-item ledger; #71 is OUT of scope as review verdict
quality. Out of scope: governance snapshot-cache redesign; #59 item 3 run-result framing;
semantic re-definition of any skill; pure live-recompute for intake/research evidence.

## Verification Verdict
S2 execution and review found and repaired these important gaps before final verification:
- #70 display boundary: `wave-plan.generated_at` no longer enters freshness diagnostic
  pairs; `TestExecutionSummaryFreshnessDiagnosticsDoesNotUseWavePlanGeneratedAt`
  covers the boundary.
- REQ-004 feature-active missing-entry boundary: a digest file that lacks an already
  accepted skill entry is stale in both read-only evaluation and mutating stamp paths,
  while first host verdict acceptance remains stampable by the next mutating run;
  `TestFeatureActiveMissingDigestEntryBlocksOnlyPreviouslyAcceptedSkill` and
  `TestStampPassingSkillDigestsBlocksFeatureActiveMissingPreviouslyAcceptedResearchDigest`
  cover the boundary.
- Historical accepted evidence: mutating acceptance now backfills lifecycle-recorded
  passing skill records only for legacy file-absent digest state after the verdict safety
  gate passes; feature-active missing entries stay blocked until the skill is rerun.
- Legacy/refreshed verdict safety: `digestInputArtifactPaths` derives artifact paths from
  the same certified digest input-set for plan, review, goal, and closeout skills instead
  of only protecting `plan-audit`; `wave-orchestration` maps `runtime_task_evidence` to
  the accepted run's task evidence JSON files so legacy backfill cannot silently bless
  task evidence written after the verdict.
- Refreshed-verdict restamps now safety-check only the inputs whose content digest actually
  changed, so a checkbox-only `tasks.md` writeback or mtime bump cannot block a restamp
  whose real drift was in a different artifact.
- Previously accepted skill re-stamps now use the stored content digest as the authority
  whenever one exists; the legacy verdict-mtime safety gate is limited to file-absent
  migration/backfill. `TestStampPassingSkillDigestsUsesStoredDigestForPreviouslyAcceptedPlanAuditCheckboxWriteback`
  proves a plan-audit digest stays fresh after S2 checkbox writeback bumps `tasks.md`
  mtime without changing its semantic task plan hash.
- Direct passing skill acceptance now fail-closes when its certified digest inputs are
  unavailable instead of silently skipping the digest stamp.
  `TestStampPassingSkillDigestsBlocksDirectPassingSkillWhenDigestInputUnavailable` covers
  both the read-path and stamping-path blocker.
- `required_skill_stale` is registered as a canonical external reason code with curated
  remediation text, and docs now state that diff-class review digests certify the current
  working diff; a commit between review and finalization may transiently stale read-only
  projections until mutating advancement restamps or the review is rerun at the new diff
  boundary.
- Diff-class review digest inputs now exclude Slipway governed bundles under
  `artifacts/changes/**`; plan-audit and final-closeout own governed artifact freshness,
  and parallel active/archive bundles no longer stale unrelated review evidence or bulk
  archive readiness.
- Diff-class review digest inputs now apply that governed-bundle exclusion to both
  workspace file discovery and execution-summary changed/target content paths. The
  regression proves `artifacts/changes/**` cannot re-enter spec/code/security/independent
  review digests through execution-summary metadata.
- Diff-class review digest inputs no longer include the full generated
  `execution-summary.yaml` artifact; review digests certify the reviewable diff and
  execution-summary changed/target content paths, so runtime summary rewrites cannot stale
  spec/code/security/independent reviews by themselves.
- Research-orchestration digest inputs now certify both `intent.md` and `research.md`, so
  substantive discovery changes stale `G_scope` instead of leaving research evidence
  silently stale.
- Legacy/refreshed verdict backfill now treats deleted certified review inputs as stale
  even when the path no longer resolves in the workspace. The deletion path is represented
  as the same synthetic deleted input digest used by normal comparison, preventing a
  missing file from being skipped during backfill.
- Goal-verification and final-closeout now use that same deleted-input sentinel for
  missing changed/target files and unmatched globs, so S4 digest diagnostics name the
  deleted artifact instead of collapsing to `input_digest_unavailable`.
- Review input policy: `security-review` and `independent-review` are included in the
  diff-class digest policy alongside spec/code reviews.
- Wave-orchestration self-output loop: `wave-orchestration` now digests semantic
  `wave-plan.yaml` structure plus parsed runtime task evidence and explicitly excludes
  the `execution-summary.yaml` it regenerates; the regression test prevents a
  stale/restamp loop.
- Artifact-clock cleanup: dead `failClosed*` helpers were deleted, the unused
  `tasksPlanUpdatedAt` parameter was removed, `execution_repair.go` no longer has a
  legacy `tasks.md` mtime fallback, and the guard test now scans production progression
  files plus the context/execution-summary/execution-repair freshness boundary.
- Compatibility boundary: `EvaluateEvidenceFreshness` still serializes legacy timestamp
  fields, but timestamp ordering is no longer a generic freshness fallback. Fresh/stale
  decisions now require structural input comparison, and timestamp ordering remains only
  at domain-specific gates such as closeout proof ordering and the explicit verdict
  safety-gate helper.
- S4 closeout boundary: `final-closeout` now has a dedicated digest input path that adds
  `assurance.md` even when the file is absent from execution-summary changed/target
  evidence; `TestFinalCloseoutInputDigestIncludesAssuranceEvenWhenNotSummarized` proves
  that a post-closeout assurance edit stales final-closeout while goal-verification stays
  fresh.
- Goal/final content boundary: `goal-verification` and `final-closeout` now digest the
  semantic changed/target path set plus the corresponding file contents, and do not digest
  the full generated `execution-summary.yaml`, `captured_at`, `run_summary_version`, or
  `tasks_plan_hash`; `TestGoalAndFinalCloseoutInputDigestExcludesExecutionSummaryMetadata`
  proves a runtime summary rewrite with unchanged changed/target content stays fresh.

Current command proof after those repairs:
- `gofmt -l` on touched Go files passed with no output.
- `golangci-lint run ./...` passed with 0 issues.
- `git diff --check` passed.
- `go build ./...` passed.
- `go vet ./...` passed.
- `go test ./...` passed.

S4 `goal-verification` and `final-closeout` are refreshed after the final evidence chain
is regenerated from this current bundle.

## Evidence Index
Current proof set:
- Focused digest/freshness regressions cover content digest comparison, refreshed verdict
  replacement, guarded legacy backfill across plan/review/goal inputs, feature-active
  missing entries, semantic `tasks_plan_hash`, generated_at display-only diagnostics,
  runtime task evidence drift in the legacy safety gate, refreshed-verdict unchanged-task
  mtimes, re-stamp checkbox writeback protection, digest-input-unavailable fail-closed
  acceptance, legacy accepted-skill backfill, feature-active missing-entry stamp blocking,
  final-closeout `assurance.md`
  binding, wave-orchestration self-output exclusion, governed-bundle exclusion for
  diff-class review digests including execution-summary content paths, research.md
  discovery binding, deleted-file legacy backfill refusal, goal/final summary-metadata
  exclusion, and diff-class review coverage for spec/code/security/independent reviews.
- Full `go test ./...`, `go build ./...`, `go vet ./...`, `gofmt -l`, and
  `git diff --check` pass after review repairs.
- `slipway validate --json` freshness is checked after the regenerated S1/S2/S3/S4
  evidence chain; final closeout records that active validation result.

## Requirement Coverage
- REQ-001 -> t-01, t-04, t-05
- REQ-002 -> t-01, t-04, t-06
- REQ-003 -> t-01, t-06, t-08, t-09
- REQ-004 -> t-01, t-05
- REQ-005 -> t-02, t-05
- REQ-006 -> t-03, t-07
- REQ-007 -> t-08, t-09, domain-review evidence
- REQ-008 -> t-01, t-06, t-08, t-09

## Residual Risks and Exceptions
Accepted scope exceptions: #59 implementation is split out and #59 remains open; #71 is
not part of this freshness/remediation thesis. Tracked risks (mitigations in decision.md
§Risk): dual acceptance routes (stamp at both and include lifecycle-recorded accepted
skills), tasks.md semantic-hash discipline, completeness of the artifact-clock sweep
(guard test), guarded legacy/refreshed-verdict restamps using a one-time verdict timestamp
safety gate, `generated_at` display-only discipline, wave-orchestration self-output
exclusion, and diff-class untracked sensitivity.

## Rollback Readiness
Source-only rollback: revert the PR. The change is additive (new model type + state
accessors + stamping helper + remediation strings) plus deletion of dead artifact-clock
code; no destructive data operation. Orphan
`evidence-digests.yaml` files are gitignored and ignored by the prior binary. Guardrail
`rollback_required` is satisfied by revert-ability.

## Archive Decision
Not ready to archive at plan time. Ready for PR only after S2 execution, S3 reviews, S4
goal-verification + final-closeout, and active `slipway validate --json` freshness proof.
Active validation is captured before done-ready; the archive (`slipway done`) step remains
a post-review/merge decision and will not describe an archived bundle as revalidated
through the active validate gate.
