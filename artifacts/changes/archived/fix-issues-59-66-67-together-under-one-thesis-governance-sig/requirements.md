# Requirements

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: EngineStampedContentDigest
REQ-001: At every mutating verdict-acceptance site — the required-skill block in
`AdvanceGoverned` (S1→S2) and the S3/S4 auto-pass authority paths — the engine MUST
compute the content digest of each accepted skill's certified input set and persist it
to an engine-owned, gitignored `verification/evidence-digests.yaml`. The host
`VerificationRecord` and `change.yaml` schemas MUST NOT change, and read-only
projections MUST NOT write the digest file. Traces to INT-001.

#### Scenario: Digest stamped on plan-audit acceptance
GIVEN a passing `plan-audit` verdict and a complete planning bundle
WHEN `slipway run` advances S1_PLAN→S2_EXECUTE
THEN `verification/evidence-digests.yaml` records a `plan-audit` entry whose `inputs`
map covers `intent.md`, `requirements.md`, `decision.md`, `research.md` when present,
`tasks.md`, and `assurance.md`, and neither the host verdict YAML nor `change.yaml`
gains new fields.

#### Scenario: Digest stamped through the auto-pass route
GIVEN passing review and verification verdicts at S3/S4
WHEN `slipway run` advances via the auto-pass authority path
THEN `goal-verification`, `final-closeout`, and review skills each receive a stored
digest entry, proving the stamping hook covers the non-`evaluateRequiredSkills` route.

### Requirement: SemanticTaskFreshness
REQ-002: `tasks.md` freshness MUST be evaluated through `wave.TaskPlanSemanticHash`
(checkbox- and format-invariant), never a raw-byte hash, so that mechanical writeback
and filesystem-only changes do not invalidate planning evidence. Traces to INT-001.

#### Scenario: Checkbox writeback stays fresh
GIVEN a passing `plan-audit` digest
WHEN `tasks.md` checkboxes are flipped `[ ]`→`[x]` by wave sync writeback
THEN the stored tasks digest is unchanged and `plan-audit` evidence remains fresh.

#### Scenario: mtime bump stays fresh
GIVEN a passing `plan-audit` digest
WHEN `git restore` or a formatter bumps a planning artifact's filesystem mtime with no
content change
THEN evidence freshness is unaffected because no comparison consults mtime.

### Requirement: ContentStalenessNamesArtifact
REQ-003: A real content change to a certified artifact MUST mark the owning skill's
evidence stale and name the changed artifact in the blocker/diagnostic, including
`assurance.md` for `plan-audit`. NO steady-state evidence-freshness code path may
consult filesystem `ModTime()` or wall-clock `now` (the `wave-orchestration` embedded
logical `CapturedAt`/`run_version` binding and the legacy file-absent migration safety
gate are the only carved-out exceptions). Traces to INT-001.

#### Scenario: Edited artifact marks evidence stale by name
GIVEN a passing skill verdict with a stored digest
WHEN a certified artifact's content is edited, including `assurance.md` for `plan-audit`
THEN `EvidenceFreshness` reports stale and the changed artifact is named in the surfaced
blocker, and re-running the skill and `slipway run` re-stamps a fresh digest.

#### Scenario: No mtime or wall-clock freshness comparison remains
GIVEN the full repository after the change
WHEN evidence-freshness paths are inspected
THEN every closeout-reuse, stale-planning, and `EvaluateEvidenceFreshness` steady-state
time/mtime branch is replaced by digest comparison, verified by a guard test that
explicitly excludes only logical `CapturedAt`, closeout proof-ordering gates, and the
legacy migration safety gate.

#### Scenario: Wave orchestration digest excludes its generated summary
GIVEN a passing `wave-orchestration` verdict
WHEN the engine computes the accepted skill digest
THEN the digest input set includes semantic `wave-plan.yaml` structure and parsed runtime
task evidence for the accepted run version, but MUST NOT include the
`execution-summary.yaml` artifact that `wave-orchestration` regenerates.

### Requirement: GuardedSilentBackfillMigration
REQ-004: Migration MUST be guarded silent backfill, bounded and observable. A LEGACY
change (no `evidence-digests.yaml` present) with already-accepted passing skills MUST
read fresh and materialize current input-set digests once on the next `slipway run` only
when a one-time verdict-timestamp safety gate passes: if any certified artifact's
filesystem mtime is after that skill verdict's timestamp, the skill MUST NOT backfill and
MUST require re-verification. A FEATURE-ACTIVE change (digest file present) whose stored
map lacks an entry for an already-accepted skill MUST treat that skill as NOT fresh. No
steady-state wall-clock/mtime fallback may remain after migration. Traces to INT-001.

#### Scenario: Legacy change self-heals once with an event when safety gate passes
GIVEN an in-flight change created before this feature, with no `evidence-digests.yaml`
AND all certified artifacts are not newer than the accepted verdict timestamp
WHEN a read-only projection evaluates freshness, then `slipway run` advances
THEN already-accepted skills read fresh, the next `slipway run` writes their digests, and
a `digest_backfilled_from_legacy_verdict` event is recorded.

#### Scenario: Legacy drift after verdict refuses backfill
GIVEN a legacy change with a passing verdict but no `evidence-digests.yaml`
AND a certified artifact has filesystem mtime after that verdict timestamp
WHEN freshness is evaluated or `slipway run` attempts migration
THEN the skill is reported stale/not fresh and must be re-verified instead of silently
locking the drifted content as the certified baseline.

#### Scenario: Runtime task evidence is protected by the legacy safety gate
GIVEN a legacy change with a passing `wave-orchestration` verdict but no
`evidence-digests.yaml`
AND a runtime task evidence JSON file for that accepted run is newer than the
`wave-orchestration` verdict
WHEN freshness is evaluated or `slipway run` attempts migration
THEN `wave-orchestration` is reported stale by name (`runtime_task_evidence`) and must be
re-verified instead of silently locking the newer task evidence as the certified baseline.

#### Scenario: Refreshed verdict restamp checks only digest-changed inputs
GIVEN a stored digest is stale because a real certified input changed
AND the host writes a newer passing verdict after that real input change
WHEN an unchanged `tasks.md` receives a checkbox-only writeback or mtime bump after the
new verdict
THEN the restamp window checks only inputs whose digest changed, so the unchanged semantic
tasks input does not block the required restamp.

#### Scenario: Feature-active missing entry is not silently fresh
GIVEN a change whose `evidence-digests.yaml` exists but has no entry for an
already-accepted skill
WHEN freshness is evaluated
THEN that skill is reported NOT fresh rather than backfilled, so backfill cannot mask a
genuinely unstamped verdict.

### Requirement: DiffInputSetPolicy
REQ-005: The certified input-set policy MUST be explicit and tested. Diff-class reviews
(`spec-compliance-review`, `code-quality-review`, `security-review`,
`independent-review`) MUST include non-ignored untracked reviewable files because those
reviews certify the current working diff; ignored files and Slipway runtime/verification
evidence plus governed bundles under `artifacts/changes/**` MUST be excluded.
`goal-verification` MUST key on the execution-summary changed and target file set
(including a semantic path-set digest for empty/non-empty sets) plus the corresponding
file contents, not the full `execution-summary.yaml` artifact or its `captured_at`
metadata, so unrelated untracked files do not stale it unless they are part of the
recorded changed/target set. Traces to INT-001.

#### Scenario: Untracked reviewable file stales diff-class review
GIVEN passing diff-class review evidence with a stored digest
WHEN a new non-ignored untracked reviewable source file appears in the workspace
THEN the corresponding review evidence is stale and the untracked file is named.

#### Scenario: Unrelated untracked file does not stale goal-verification
GIVEN passing `goal-verification` evidence with a stored digest
WHEN an unrelated untracked file appears outside the execution-summary changed/target set
THEN `goal-verification` remains fresh; if that file is later recorded as changed/target
evidence, it becomes part of the certified input set.

#### Scenario: Runtime summary rewrite does not stale goal-verification
GIVEN passing `goal-verification` evidence with a stored digest
AND the execution-summary changed/target file set and file contents are unchanged
WHEN `execution-summary.yaml` is regenerated with a different `captured_at`
THEN `goal-verification` remains fresh because the full runtime summary is not a digest
input.

#### Scenario: Runtime evidence files are excluded from diff review digest
GIVEN passing diff-class review evidence with a stored digest
WHEN Slipway writes ignored runtime evidence or verification summary files
THEN those files do not stale the diff-class review digest.

### Requirement: S4RecoveryRemediationRouting
REQ-006: The S4 post-review recovery path MUST be self-describing: the
`evidence_task_wrong_state`, `verification_evidence_missing`, and
`closeout_goal_verification_reuse_invalid` surfaces MUST route the operator to refresh
evidence by re-running `goal-verification` then `final-closeout` (task evidence is
S2-only), and, when digest diagnostics are available via INT-001, name the artifact that
invalidated the evidence. Traces to INT-003.

#### Scenario: evidence task in S4 points to the supported path
GIVEN a change in `S4_VERIFY`
WHEN the operator runs `slipway evidence task`
THEN the `evidence_task_wrong_state` remediation states task evidence is S2-only and
directs re-running goal-verification + final-closeout to refresh S4 evidence.

#### Scenario: Stale S4 blocker names refresh path and artifact
GIVEN an S4 source/test edit invalidated `goal-verification` evidence
WHEN `slipway validate --json` renders the blocked state
THEN the blocker remediation names the changed artifact and the supported refresh path.

### Requirement: ProofAndDomainReview
REQ-007: The change MUST ship with focused regression tests for each acceptance signal
plus full `go build ./...`, `go test ./...`, and `slipway validate --json` proof, and
MUST carry domain-aware review evidence because it touches the `schema_data_migration`
guardrail (evidence/runtime schema + migration). Traces to INT-001.

#### Scenario: Verification commands pass
GIVEN implementation and tests are complete
WHEN verification runs
THEN focused regressions, `go build ./...`, `go test ./...`, and `slipway validate
--json` provide fresh passing proof, and domain review records the intentional
schema/migration change.

### Requirement: WavePlanDisplayTimestampNotFreshnessInput
REQ-008: The wave-plan / stale-planning recovery chain MUST be content-based: a
re-materialized `wave-plan.yaml` MUST NOT carry a `generated_at` derived from `tasks.md`
filesystem mtime. `generated_at` MAY be the actual/injected wave-plan materialization
timestamp, but it MUST be display/audit metadata only; no freshness comparison may consume
it. `internal/state/execution_summary.go` MUST key the stale-planning chain on
`tasks_plan_hash` (semantic) rather than `generated_at` timestamp ordering, so a refreshed
`plan-audit` whose task content is unchanged cannot leave the chain permanently stale
after S4 recovery. Traces to INT-004.

#### Scenario: Refreshed plan-audit with unchanged tasks does not strand the chain
GIVEN S4 stale-planning recovery reopened S1 and accepted a fresh `plan-audit`
AND `tasks.md` content (and its `tasks_plan_hash`) is unchanged
WHEN `slipway run` re-materializes the wave plan and rebuilds the execution summary
THEN `slipway validate --json` reports the planning chain fresh (no
`stale_planning_evidence`) without any manual edit to `wave-plan.yaml.generated_at`.

#### Scenario: Wave-plan timestamp is display-only
GIVEN a regenerated `wave-plan.yaml`
WHEN its freshness is evaluated against the planning evidence
THEN the comparison does not treat `tasks.md` mtime or `wave-plan.generated_at` as the
wave-plan freshness authority; only the semantic `tasks_plan_hash` drives the planning
chain.
