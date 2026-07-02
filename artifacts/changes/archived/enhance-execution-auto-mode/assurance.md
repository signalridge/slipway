# Assurance

## Scope Summary

This change implements bounded auto-to-next-real-gate behavior for
`execution.auto` / `slipway run --auto`.

Delivered scope:

- `run` and stage command loops now receive the effective auto setting when
  deciding whether to stop.
- Auto mode may continue only routine `run_slipway_run_to_advance` command
  boundaries after a successful advance.
- Auto mode still stops for `next_skill`, review batches, non-pacing blockers,
  `done_ready`, `StateDone`, and noop/no-advance views.
- Public docs, localized command docs, config catalog text, and generated command
  notes now describe bounded auto behavior instead of full automation.

Out of scope:

- No embedded governance skill executor.
- No automatic review dispatch.
- No synthetic evidence generation.
- No automatic done finalization.

## Verification Verdict

Implementation verification passed for the bounded auto behavior. Terminal ship
verification records a fresh transcript at
`verification/logs/ship-suite.txt`; the closeout suite covers whitespace checks,
targeted auto-mode regressions, generated-surface checks, full Go tests, lint,
stale wording scans, target-file placeholder scans, and the current lifecycle
readiness checks.

## Evidence Index

- `cmd/run.go`: threads the effective auto flag into the governed loop and adds
  the narrow routine-boundary continuation predicate, including an explicit
  guardrail-domain fail-closed check.
- `cmd/next.go` and `cmd/next_skill_view.go`: document the invariant that
  `run_slipway_run_to_advance` must remain a pure routine boundary and must not
  co-occur with non-pacing blockers.
- `cmd/stage.go`: stage loops use the same auto-aware stop predicate.
- `cmd/auto_mode_test.go`: covers routine boundary continuation, manual pacing,
  loop-level auto continuation, guardrail fail-closed behavior, and hard-stop
  preservation.
- `cmd/progression_next_test.go`: updated existing stop-predicate coverage.
- README and command reference docs: describe bounded auto behavior and explicit
  non-goals, including that non-sensitive handoffs may report
  `evidence_continuation` while run/stage loops still stop for host work.
- `internal/toolgen/toolgen.go`, `internal/model/config.go`, and
  `internal/model/config_catalog.go`: generated/help/config text aligned with
  bounded auto semantics.
- `internal/tmpl/templates/_partials/command-*.tmpl` and
  `internal/tmpl/templates_test.go`: generated command prompt bodies now state
  the JSON/loop distinction and tests pin the wording.
- `verification/wave-orchestration.yaml`: records S2 execution evidence for
  tasks `t-01` through `t-04`.
- Runtime task evidence under `.git/slipway/runtime/changes/enhance-execution-auto-mode/evidence/tasks/`.

Verification commands recorded during execution:

- `go test ./cmd -run 'Test(ShouldStopRunLoopAuto|RunGovernedLoopWithBuilder|ShouldStopRunLoopDoesNotStopForExecutionResumeContext|ResolveEffectiveAuto|RunCmdRejectsBothAutoAndNoAuto|DeriveConfirmationRequirementAuto|NextPreviewUnderAuto|ConfigAutoReachesStageAndHookEntries)'`
- `go test ./internal/engine/progression -run 'Test(AdvanceGoverned_Auto|SecurityReviewDivergesAcrossAutoBoundaries|SkillRequiresManualAutoBoundary|PurePacingAutoSafeAllowlistMembership)'`
- `go test ./internal/model ./internal/toolgen ./internal/tmpl`
- `git diff --check`
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
- stale wording `rg` scan across README, docs, cmd, and internal Go files.
- `go test ./...`
- `golangci-lint run --timeout 5m ./...`
- `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate`
- `SLIPWAY_HOST_CAPABILITIES=subagent go run . next --json --diagnostics`

## Requirement Coverage

- REQ-001: covered by the auto-aware loop predicate and
  `TestShouldStopRunLoopAutoContinuesRoutineRunBoundary` /
  `TestRunGovernedLoopWithBuilderAutoContinuesRoutineRunBoundary`.
- REQ-002: covered by
  `TestRunGovernedLoopWithBuilderManualStopsAtRoutineRunBoundary`.
- REQ-003: covered by hard-stop cases for `NextSkill` and review batch in
  `TestShouldStopRunLoopAutoKeepsRealStops`, and by generated command text that
  explains `evidence_continuation` is standing host authorization, not loop
  advancement.
- REQ-004: covered by hard-stop cases for non-pacing blockers, `done_ready`,
  guardrail domain, noop/no-advance, and existing auto boundary tests.
- REQ-005: covered by README/reference/localized docs updates, generated command
  notes, command prompt partials, config catalog text, and stale wording scan.

## Residual Risks and Exceptions

- S3 review and terminal ship verification used host-native fresh subagent
  contexts. Each selected reviewer records a distinct
  `context_origin:stage=review=<handle>`; no degraded fallback is used for this
  closeout.
- The placeholder scan reports expected non-production matches:
  `internal/tmpl/templates_test.go` intentionally asserts generated placeholder
  scan text, `assurance.md` documents the prior scan result, and the
  `return nil, nil` matches in `cmd/next.go` / `internal/toolgen/toolgen.go`
  are valid empty-selection/no-op-result returns, not unimplemented production
  behavior.

## Rollback Readiness

Rollback is a source revert of the touched run/stage loop logic, tests, docs,
config/help text, and governed artifacts for this change. There are no durable
schema migrations or runtime state format changes.

## Archive Decision

Archive is ready after terminal ship-verification evidence is accepted by the
active lifecycle gate and `slipway run` advances the change to `done_ready`.
Active `validate` freshness was captured in
`verification/logs/ship-suite.txt`; archived bundles are frozen records and are
not revalidated through the active validate gate after archival.
