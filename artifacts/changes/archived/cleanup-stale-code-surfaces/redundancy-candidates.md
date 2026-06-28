# Redundancy Candidate Inventory

This file is the durable execution checklist for the pasted-report cleanup
scope. It replaces chat-only scope as the source the S2 executors and final
assurance must audit against.

Rules for every candidate:

- Verify the source anchor or tool-output requirement in the current governed
  worktree before changing code.
- Do not use old worktrees as deletion or retention evidence.
- Record one disposition for each candidate in the owning task result:
  `removed`, `consolidated`, `preserved_live`, or
  `not_applicable_with_evidence`.
- A `preserved_live` disposition must name the current production caller,
  runtime path, config path, generated-surface contract, or tool output that
  proves the candidate is still live.
- If a candidate is live but the allowed prove-still-live outcome below does
  not cover that result, stop for plan amendment instead of widening scope.

## Candidates

### C-001: Test-held wrappers and no-consumer internal API

- Source anchor or tool output: current-worktree source references in
  `cmd/*`, `internal/engine/progression/*`, `internal/state/*`,
  `internal/toolgen/*`, and focused `unused --tests=false` findings; includes
  `ResumeWaveIndexFromTaskEvidence` ownership in
  `internal/engine/progression/wave_sync.go`.
- Owning task: `t-01`.
- Expected action: remove or inline wrappers/helpers kept alive only by tests,
  then redirect tests to the production helper that owns behavior.
- Allowed prove-still-live outcome: preserve only if a current non-test
  production caller or runtime path is recorded in `task-result-t-01.json`.

### C-002: Focused unused cleanup findings

- Source anchor or tool output:
  `golangci-lint run --enable-only=unused --tests=false ./...`.
- Owning task: `t-01`.
- Expected action: remove or inline every targeted unused production surface.
- Allowed prove-still-live outcome: preserve only with current source evidence
  showing the linter candidate is outside the intended cleanup set or is kept
  live by production code.

### C-003: Dead model, state, and capability fields or methods

- Source anchor or tool output: current source references under
  `internal/model`, `internal/state`, `internal/engine/capability`,
  `internal/engine/skill`, and `cmd/tool_github.go`.
- Owning task: `t-02`.
- Expected action: remove fields or methods with no current writer, reader, or
  public behavior contract, and update stale tests/snapshots.
- Allowed prove-still-live outcome: preserve only with current writer/reader
  evidence and a reason that the field or method is part of live behavior.

### C-004: Inert `CloseoutConditional` required-skill filtering

- Source anchor or tool output:
  `rg -n "closeout_conditional|CloseoutConditional" .slipway.yaml internal/tmpl/templates internal/engine`.
  Plan repair confirmed current readers in `internal/engine/skill/skill.go`,
  `internal/engine/progression/authority.go`, and
  `internal/engine/progression/evidence_repair.go`, with no current
  registry/preset/template setter found by that search.
- Owning task: `t-03`.
- Expected action: remove the field, closeout-required parameter threading, and
  tests that assert the old split, without weakening live ship-verification
  enforcement.
- Allowed prove-still-live outcome: if any current registry, preset, template,
  or config path sets `closeout_conditional: true`, stop for plan amendment
  rather than removing it.

### C-005: Narrow retired readers, obsolete shims, and legacy handoff hygiene

- Source anchor or tool output: current source references in
  `internal/state`, `internal/engine/progression`, `cmd/evidence.go`,
  `cmd/next_skill_view.go`, `cmd/stats.go`, `cmd/status_view_build.go`,
  `cmd/repair.go`, and related tests.
- Owning task: `t-03`.
- Expected action: remove compatibility-only readers and obsolete shims while
  preserving fail-closed retired-input rejection.
- Allowed prove-still-live outcome: preserve only if a current runtime path
  still uses the reader for live data, and record the path.

### C-006: Retired workflow-state normalization

- Source anchor or tool output:
  `rg -n "S2_EXECUTE|S4_VERIFY" artifacts/changes` plus current source
  references in `internal/model`, `internal/state`, `cmd/status_view_build.go`,
  and status timeline tests.
- Owning task: `t-03`.
- Expected action: remove `S2_EXECUTE`/`S4_VERIFY` canonicalization and tests
  that preserve that compatibility as an intentional behavior change.
- Allowed prove-still-live outcome: if an active current-worktree change or
  runtime record still requires those states, stop for plan amendment instead
  of silently preserving compatibility.

### C-007: No-longer-emitted reason codes and remediations

- Source anchor or tool output: current gate emission search for the candidate
  reason codes, plus `internal/model/reason_code.go`,
  `internal/model/recovery.go`, and focused command/progression tests.
- Owning task: `t-04`.
- Expected action: remove stale catalog, remediation, frozen expectation, and
  fabricated stale-code tests.
- Allowed prove-still-live outcome: preserve only if a current gate still emits
  the reason code and the emitting path is recorded.

### C-008: Focused `unparam`, `staticcheck`, `ineffassign`, and `wastedassign` cleanup

- Source anchor or tool output:
  `golangci-lint run --enable-only=unparam --tests=false ./...` and
  `golangci-lint run --enable-only=staticcheck,ineffassign,wastedassign --tests=false ./...`.
- Owning task: `t-05`.
- Expected action: resolve each targeted assignment, parameter, and duplicate
  wiring issue.
- Allowed prove-still-live outcome: preserve only if the current focused lint
  run no longer reports the candidate or if source evidence shows the finding
  is intentionally outside this cleanup scope.

### C-009: Low-risk duplicate command path-authority wiring

- Source anchor or tool output: current source references in `cmd/fix.go`,
  `cmd/freshness_diagnostics.go`, `cmd/next_skill_view.go`,
  `cmd/next_wave_plan.go`, `cmd/progression_next_test.go`, and
  `cmd/tool_github.go`.
- Owning task: `t-05`.
- Expected action: consolidate identical duplicate wiring where behavior is
  already covered.
- Allowed prove-still-live outcome: preserve only with a recorded behavioral
  difference or test contract that makes consolidation unsafe.

### C-010: Validation config no-op flags

- Source anchor or tool output: config decode/catalog/get/set references in
  `internal/model/config.go`, `internal/model/config_catalog.go`,
  `cmd/config_test.go`, and progression validation tests.
- Owning task: `t-06`.
- Expected action: remove no-op validation config flags and stale catalog or
  config command tests.
- Allowed prove-still-live outcome: preserve only if current requirements
  enforcement changes when the flag value changes, with a focused test.

### C-011: Write-only review drift counters

- Source anchor or tool output: current source references in `cmd/review.go`,
  `cmd/review_test.go`, and related state/model fields.
- Owning task: `t-06`.
- Expected action: remove write-only counters and tests that only preserve
  them.
- Allowed prove-still-live outcome: preserve only with a current reader or
  public output contract.

### C-012: Public no-op `done --json` and `validate --json` flags

- Source anchor or tool output: `README.md` currently documents both flags;
  `internal/toolgen/surface_manifest.go` registers `README.md` as a checked
  public surface; command metadata, `docs/reference/commands.md`, and
  `docs/SURFACE-MANIFEST.json` must stay synchronized.
- Owning task: `t-06`.
- Expected action: retire the no-op flags from command code, tests, README,
  reference docs, and generated surface metadata.
- Allowed prove-still-live outcome: if either flag currently changes command
  semantics, stop for plan amendment rather than treating it as no-op cleanup.

### C-013: Live artifact schema behavior, including `custom_artifacts`

- Source anchor or tool output:
  `rg -n "custom_artifacts|ArtifactSchemaCustom|artifact_schema|custom" .slipway.yaml internal/model internal/engine cmd`.
  Current worktree references show config decode/encode, validation, catalog,
  bundle scaffolding, readiness, instructions, and tests for `core`,
  `expanded`, and `custom`.
- Owning task: `t-06` and final assurance `t-13`.
- Expected action: preserve live `core`, `expanded`, and `custom` artifact
  schema behavior and keep tests synchronized.
- Allowed prove-still-live outcome: this is a preservation candidate. Removing
  `custom_artifacts` or the `custom` schema is out of scope and requires a
  separate product decision.

### C-014: Command route and freshness duplication

- Source anchor or tool output: current source references in
  `cmd/freshness_diagnostics.go`, `cmd/status_view_build.go`,
  `cmd/status.go`, `cmd/status_render.go`, `cmd/common.go`, `cmd/validate.go`,
  `cmd/next.go`, `cmd/next_handoff.go`, `cmd/done.go`, and related tests.
- Owning task: `t-07`.
- Expected action: consolidate `statusRoute` vs route-kind overlap and
  `EvidenceFreshness` vs `ExecutionEvidenceFreshness` synchronization.
- Allowed prove-still-live outcome: preserve only with recorded public-output
  or lifecycle-readiness evidence showing the apparent duplication carries
  distinct behavior.

### C-015: `cmd/tool_github` helper duplication

- Source anchor or tool output: current source references in
  `cmd/tool_github.go` for pagination, check-run envelope, and status
  extraction helpers.
- Owning task: `t-08`.
- Expected action: share backend-agnostic helper behavior and add
  `cmd/tool_github_test.go` if needed for focused coverage.
- Allowed prove-still-live outcome: preserve only with recorded current
  GitHub-output semantics that require separate helpers.

### C-016: Stale evidence repair predicate duplication

- Source anchor or tool output: current source references in
  `internal/engine/progression/evidence_repair.go`,
  `internal/engine/progression/readiness.go`,
  `internal/state/execution_summary.go`, `internal/state/evidence_digests.go`,
  and `internal/state/wave_execution.go`.
- Owning task: `t-09`.
- Expected action: consolidate repeated stale-evidence predicates without
  weakening fail-closed recovery.
- Allowed prove-still-live outcome: preserve only if the predicates differ in a
  current behavior that tests assert.

### C-017: Artifact contract helper boilerplate

- Source anchor or tool output: current source references in
  `internal/engine/artifact/decision_contract.go`,
  `internal/engine/artifact/requirements_contract.go`, and
  `internal/engine/artifact/tasks_contract.go`.
- Owning task: `t-09`.
- Expected action: consolidate repeated read/empty/error handling while keeping
  strict artifact validation.
- Allowed prove-still-live outcome: preserve only if a contract has a distinct
  user-facing error or validation behavior that focused tests cover.

### C-018: Strict cache loaders and load-error wrappers

- Source anchor or tool output: current source references in state/progression
  cache loaders and associated strict decode tests.
- Owning task: `t-09`.
- Expected action: consolidate repeated strict loader or load-error wrapper
  logic without weakening corrupt-cache diagnostics.
- Allowed prove-still-live outcome: preserve only with a current distinct error
  contract or cache format difference.

### C-019: `blockerRemediations` vs `canonicalReasonDefinitions` drift

- Source anchor or tool output: current source references in
  `internal/model/reason_code.go`, `internal/model/recovery.go`, and contract
  tests.
- Owning task: `t-09`.
- Expected action: establish one ownership or a completeness check so reason
  definitions and remediation entries do not drift.
- Allowed prove-still-live outcome: preserve only if a focused test proves the
  two tables intentionally have different domains.

### C-020: S3 review template repeated text

- Source anchor or tool output: current source references in
  `internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl`,
  `independent-review/SKILL.md.tmpl`, `security-review/SKILL.md.tmpl`,
  `spec-compliance-review/SKILL.md.tmpl`, and `internal/tmpl/templates_test.go`.
- Owning task: `t-10`.
- Expected action: extract repeated disk-handoff and record-verification
  contracts into `_partials/review-disk-handoff.tmpl` and
  `_partials/review-record-verification.tmpl`, or prove extraction is unsafe.
- Allowed prove-still-live outcome: preserve only if template-specific
  differences make a shared partial misleading, with rendered-template evidence.

### C-021: Verification test helper duplication

- Source anchor or tool output: current source references in
  `internal/engine/governance/test_helpers_test.go`,
  `internal/engine/governance/runtime_actions_test.go`,
  `internal/engine/progression/test_helpers_test.go`, and
  `internal/engine/progression/advance_governed_test.go`.
- Owning task: `t-11`.
- Expected action: consolidate repeated verification-writing helpers while
  preserving coverage.
- Allowed prove-still-live outcome: preserve only if package boundaries or test
  behavior make sharing unsafe, with a focused test reference.

### C-022: Tiny command `findRepoRoot` duplication

- Source anchor or tool output: current source references in
  `internal/toolgen/cmd/gen-surface-manifest/main.go` and
  `internal/coverage/cmd/covergate/main.go`.
- Owning task: `t-12`.
- Expected action: create `internal/fsutil/repo_root.go` and
  `internal/fsutil/repo_root_test.go` if shared root discovery is implemented,
  then migrate both tiny binaries.
- Allowed prove-still-live outcome: preserve only if the two binaries require
  different root-discovery semantics, with current-root behavior evidence.

## Advisory Context

- `artifacts/codebase` is populated but scope-stale for this cleanup. It may be
  read for broad orientation only. Each executor must re-derive task-local
  seams and callers from current source before deletion or retention.
- Source/test/template files listed in `tasks.md` that do not exist at plan
  time are intentional planned outputs only for the consolidation path named by
  the owning task. If the candidate is instead proven still live, the task
  result must state that the file was not created and why.
