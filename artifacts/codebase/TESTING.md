# Testing

Re-authored for change
`add-an-engine-consumed-context-origin-fresh-context-attestat`.

## Existing Coverage

- `internal/engine/progression/authority_test.go` holds the clonable gate
  blueprints. `TestCloseoutAssuranceAttestationBlockers` is the Pattern-A
  presence-attestation unit shape (record map in, blocker-or-empty out, padded
  reference tolerated). `TestBuildShipAuthorityAttestationPresetGating` is the
  three-subtest preset-gating table proving fail-closed on plain standard,
  surfacing in both `ship.VerifySkillBlockers` and `ship.Result.ReasonCodes`,
  and advisory/absent on light (asserts on `policy.EffectivePreset` /
  `CloseoutRefreshRequired`) — the exact template for the P1 always-on
  chain-ordering gate. `TestCloseoutGoalVerificationReuseBlockers` and
  `TestBuildShipAuthoritySurfacesCloseoutReuseBlocker` are the Pattern-B
  cross-stage `Timestamp` + `RunVersion` + reference-token blueprint (helpers
  `passingCloseoutReuseRecords`, `closeoutReuseReviewRecords`,
  `closeoutReuseExecutionSummary*`); the ordering chain it asserts is the same
  one P1 promotes from the reuse branch to always-on.
- `internal/model/reason_code_contract_test.go`:
  `TestCanonicalReasonCodeTaxonomySnapshot` pins the full
  `canonicalReasonCodeSnapshot()` list (alphabetical) against
  `canonicalReasonDefinitions`, plus `canonicalReasonSeveritySnapshot` (all
  `ReasonSeverityError` except the small warning/info override map).
  `TestDeclaredWaveBlockerVocabularyRetired` shows the three-surface removal
  pattern (registry + `blockerRemediations` + `IsCanonicalReasonCode`).
  `TestReasonAndErrorContractTestsDoNotTextMatchMessageProse` /
  `TestMessageProseAssertionLint*` are the repo-wide AST lint forbidding
  assertions on `.Message` prose — new tests must assert `Code`/`Detail`,
  not message text.
- `internal/model/recovery_test.go`: `TestRemediationTableEntriesAreComplete`,
  `TestRecoveryRelevantTokensResolveToRemediation` (driven by
  `recoveryRelevantCanonicalCodes()` + `sampleRecoveryDetail`), and
  `TestInScopeProducedBlockersResolveToCanonicalRecovery` (via
  `inScopeProducedRecoverySpecs()`) are the completeness harness any new code
  must extend. `TestCloseoutAttestationMissingResolvesToRecovery` and
  `TestBuildRecoveryPrioritizesVerificationBeforeCloseout` are the closest
  closeout-blocker recovery precedents.
- `internal/tmpl/templates_test.go`: literal token-string assertions, one set
  per emitted token. `TestFinalCloseoutTemplateRequiresAssuranceAttestationOnStandardStrict`
  (the `closeout:assurance_complete=pass` + `closeout_assurance_attestation_missing`
  precedent), `TestFinalCloseoutSkillDocumentsGoalVerificationReuseContract`
  (the `closeout:goal_verification_reuse*` reference tokens), and
  `TestReviewTemplatesRequireNegativePathAndToolchainEvidence` /
  `TestRunSummaryBoundGovernedTemplatesDoNotUseLiteralRunVersion` cover the
  spec-compliance / code-quality review templates that P2 force-parallel touches.
- Wave-sync gates live in `internal/engine/progression/wave_sync_test.go`:
  `TestDispatchEvidenceBlockers_*` (3 cases) and `TestExecutorAgentBlockers_*`
  (5 cases) are the dispatch-mode / two-distinct-handle (#5/#6) templates;
  `TestSyncGovernedWaveExecutionRecordsDegradedDispatchMode` and
  `TestSyncGovernedWaveExecutionUsesEffectiveParallelForDispatchMode` exercise
  `degraded_sequential`. Session isolation is emitted (`wave_sync.go`
  `session_isolation_warning:session_id=...:shared_by=...`) but has no dedicated
  blocker-gate test to clone.

## Gaps Closed By This Change

- P1 chain-ordering gate: a new `authority_test.go` test cloning the
  `TestBuildShipAuthorityAttestationPresetGating` three-row shape
  (pass / fail-closed-standard+strict / advisory-on-light) for
  `closeout >= goal >= max(spec-compliance-review, code-quality-review)`,
  asserting the new distinct reason code reaches both `VerifySkillBlockers`
  and `Result.ReasonCodes`.
- P2 review force-parallel: dispatch-mode + two-distinct-`executor_agent`-handle
  tests for the review PAIR, cloned from `TestDispatchEvidenceBlockers_*` /
  `TestExecutorAgentBlockers_*`, asserting fail-closed on standard/strict,
  advisory on light, and that the two reviews are unordered peers while
  verify+closeout stay ordered after them.
- Reason-code contract: new entries in `canonicalReasonCodeSnapshot()` +
  `canonicalReasonSeveritySnapshot` for the P1 ordering code and any P2 handle
  code, plus matching `blockerRemediations` / `recoveryRelevantCanonicalCodes()`
  / `sampleRecoveryDetail` rows so the recovery-completeness tests stay green.
- Template tokens: new `templates_test.go` literal-token assertions for any
  per-review context-id token P2 emits and any P1 ordering guidance added to the
  closeout/verify templates.
- #5/#6 wave_sync: a NET-NEW preset-parametrized test (no preset-parametrized
  wave_sync test exists today) covering #5 rebased on engine-owned
  `task_kind` + `target_files` (session_id demoted) and #6 requiring a paired
  genuine tool-unavailable signal alongside `degraded_sequential`.

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
