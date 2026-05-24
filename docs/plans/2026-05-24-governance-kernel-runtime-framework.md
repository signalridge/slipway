# Governance Boundary Hardening And Targeted Write-Gateway Plan

Date: 2026-05-24
Status: implemented

## Purpose

Tighten Slipway's governance boundaries with the smallest useful change set.

This replaces the broader kernel/runtime consolidation idea with a narrower
plan based on current code verification. The codebase has already paid down a
large part of the architecture debt this plan originally assumed:

- `next` is already the query surface.
- `run` is already the state-advancing execution surface.
- `runGovernedLoop` reuses `buildNextView` rather than maintaining a separate
  decision path.
- `ReasonCode` is already the shared blocker/warning shape for gates,
  readiness, verification, execution summaries, health, and command views.
- `change.yaml` is already the single current-state authority.
- worktree/orchestrator promotion is already deferred by ADR.

The goal is therefore not to add a new orchestration layer or a parallel typed
model. The goal is:

1. lock the current contracts with focused regression tests;
2. centralize the highest-value authority writes where they are actually
   concentrated;
3. defer typed host/runtime expansion until a real second consumer or migration
   pressure exists.

Implementation landed as focused contract tests/static guards plus targeted
`model.Change` authority helpers in the transition-heavy production paths named
below. Deferred reopen gates remain outside this optimization.

## Verified Baseline

These facts were checked against the current source before revising this plan.

- `cmd/next.go` defines `nextView` as the compatibility JSON output shape for
  `next` and `run`. It currently has 34 exported JSON fields. Its width is a
  public-output concern, not by itself evidence of duplicated business logic.
- `cmd/run.go` implements `runGovernedLoop` by repeatedly calling
  `buildNextView`. The plan must not claim that `cmd/next.go` and `cmd/run.go`
  maintain separate decision implementations.
- `internal/model/reason_code.go` defines `ReasonCode`, and the existing gate,
  readiness, verification, execution-summary, health, review, status, and
  command surfaces already carry blocker details through `[]ReasonCode`.
- `internal/engine/status/view.go` already defines an `EvidenceRef` type for
  status projections. A new global `model.EvidenceRef` would create naming and
  migration pressure before there is a real consumer.
- `model.Change` already has `Normalize`, `Validate`, and marshal-time
  normalization hooks. A write gateway should build on that local authority
  instead of replacing it with a new kernel object.
- Assignment-style writes to authoritative `Change` fields are concentrated
  enough to justify a targeted gateway, but they are not broad enough to justify
  a whole new state model. The highest-value implementation review starts with:
  `internal/engine/progression/advance_governed.go`,
  `internal/engine/progression/advance_intake.go`,
  `cmd/pivot_execution.go`, and the mutation-adjacent read side in
  `internal/engine/governance/runtime_actions.go`.
- `runtime_actions.go` is included in the review scope because it participates
  in governance action decisions, not because this plan has verified direct
  authority-field writes there. Only files with actual writes should be migrated
  to write helpers.

## Baseline Contract

The current accepted product contract remains the authority for this plan:

- `next` is read-only and must not mutate `change.yaml` or append lifecycle
  events.
- `run` is the explicit state-advancing execution surface.
- `change.yaml` is the current lifecycle and routing authority.
- `events/lifecycle.jsonl` is append-only trace data, not a second authority.
- `runtime-state.yaml` remains legacy compatibility only.
- policy packs are advisory and cannot weaken built-in guardrails.
- `learn --preview` is read-only; apply is unsupported today.
- first-class worktree lanes, orchestrator-managed parallel execution, and
  multi-worktree scheduling remain deferred product features.

These constraints may change later only through an explicit ADR or command
contract update with tests and migration notes in the same governed change.

## Non-Goals

This plan does not:

- add `model.Decision`;
- add `model.Finding`;
- add a new global `model.EvidenceRef`;
- extract a broad `internal/engine/decision`, `internal/engine/orchestration`,
  or `internal/app` service;
- change public command JSON;
- change `change.yaml` disk shape;
- add `schema_version`;
- split `change.yaml` into multiple current-state authorities;
- make policy packs mandatory or able to weaken built-in guardrails;
- productize worktree lanes, claim/lease semantics, or orchestrator scheduling;
- implement learning apply.

## Architecture Discipline

Keep the existing authority split:

```text
Surface
  CLI, generated tool surfaces, agent skills, human views

Learning
  read-only preview proposals today

Policy
  built-in controls, advisory policy packs, diagnostics

Runtime
  run loop, wave/task execution, checkpoint, resume, workspace binding checks

Change Authority
  change identity, lifecycle, risk, gates, evidence refs, append-only events
```

Dependency rules:

- CLI surfaces may render compatibility JSON and text.
- CLI surfaces should not invent independent governance semantics when an
  engine function already owns the behavior.
- Engine packages may request state transitions and record runtime evidence.
- Policy packages may emit advisory diagnostics and required actions.
- Learning may read `change.yaml` and lifecycle events to produce manual-review
  proposals.
- `internal/model` must not import command renderers, generated tool surfaces,
  skill prompts, policy-pack parsing internals, or learning proposal generation.

This is a direction guard, not a mandate to create one package or state file per
layer.

## Phase 0: Contract Lock

Goal: protect the existing product boundaries before touching write paths.

This phase starts by inventorying current coverage and adding only missing
tests. Do not duplicate tests that already prove the same contract.

Tasks:

- Confirm existing `next` read-only coverage:
  - state does not advance during `next`;
  - artifact reconciliation projected by `next` is not persisted;
  - `next` command-entry templates describe query-only behavior.
- Add missing precision tests where coverage is absent:
  - `next` must not append lifecycle events during inspection;
  - `learn` without a supported apply path must return
    `learn_apply_unsupported`;
  - `learn --preview` output remains `preview=true` and `auto_apply=false`.
- Confirm existing `run` contract coverage:
  - `run` exposes transition traces;
  - `runGovernedLoop` continues to reuse `buildNextView`;
  - generated `run` command entries describe loop behavior.
- Confirm policy-pack advisory coverage:
  - valid policy packs are advisory diagnostics;
  - blocking policy-pack declarations produce diagnostics and cannot weaken
    built-in guardrails.
- Add a lightweight command-surface guard:
  - public `worktree`, `lane`, or `orchestrator` commands must not be added
    without updating `docs/worktree-orchestrator-deferment.md` and
    `docs/command-contract-matrix.md`.
- Add a lightweight dependency-direction guard:
  - `internal/model` and state-authority packages must not import `cmd`,
    generated command renderers, or skill prompt/template packages.

Exit criteria:

- Current contracts are locked by tests or static guards.
- New tests are gap-filling, not duplicate coverage.
- No production behavior changes.
- No public JSON changes.
- No new Decision/Finding/EvidenceRef wrapper types.

## Phase 1: Targeted Change Authority Methods

Goal: reduce the riskiest direct writes to authoritative `model.Change` fields
without changing the serialized disk format.

This phase is deliberately narrow. It is not a full rewrite of every test
fixture or every direct assignment in the repository.

Review scope:

- `internal/engine/progression/advance_governed.go`
- `internal/engine/progression/advance_intake.go`
- `cmd/pivot_execution.go`
- `internal/engine/governance/runtime_actions.go`

Migration rule:

- Convert direct writes only where a helper captures a real authority
  transition, cleared-field convention, or evidence mutation.
- Do not migrate read-only checks.
- Do not migrate test fixtures.
- Do not add a helper just to hide a single obvious assignment unless it
  prevents an invariant violation.

Recommended helpers may live near `model.Change`, for example in
`internal/model/change_authority.go`:

```go
func (c *Change) TransitionTo(state WorkflowState) []string
func (c *Change) AdvancePlanSubStep(next PlanSubStep)
func (c *Change) EnterPlanning(needsDiscovery bool)
func (c *Change) ClearPlanningSubStep() bool
func (c *Change) ClearIntakeSubStep() bool
func (c *Change) SetActiveCheckpoint(cp ActiveCheckpoint)
func (c *Change) ClearActiveCheckpoint() bool
func (c *Change) ResetEvidenceRefs()
func (c *Change) RecordEvidenceRef(key, path string)
func (c *Change) ClearAutoPassHistory() bool
func (c *Change) ResetReviewIntentDriftFailures()
func (c *Change) MarkInterrupted(at time.Time)
func (c *Change) ClearInterruptedExecution() bool
```

The exact helper set should be smaller if implementation shows fewer helpers
are needed. Prefer domain verbs over generic setters.

Tasks:

- Inventory direct authority writes in the review scope.
- Add the minimum helper methods needed for lifecycle, planning substep,
  intake substep, checkpoint, evidence-ref, auto-pass, and pivot reset
  transitions.
- Replace direct writes in `advance_governed.go`, `advance_intake.go`, and
  `pivot_execution.go` where the helper materially improves invariants or
  cleared-field accounting.
- Touch `runtime_actions.go` only if the final inventory shows a concrete
  invariant can move into a read helper or authority method; otherwise leave it
  unchanged.
- Preserve `change.yaml` serialization exactly.
- Preserve lifecycle event shape and command JSON exactly.

Exit criteria:

- The targeted files have fewer authority writes in transition-heavy logic.
- State transitions and cleared-field behavior remain auditable.
- Existing behavior and command JSON are unchanged.
- `go test ./... -count=1` passes.

## Phase 2: Plan And Documentation Reconciliation

Goal: make the documented plan match the narrower execution scope so future
agents do not implement the retired broad plan by accident.

Tasks:

- Keep this document as the current execution plan.
- If `docs/README.md` or plan index docs mention the older kernel/runtime
  consolidation shape, update them only to point to this narrower boundary
  hardening scope.
- State explicitly that typed host contracts, runtime unification, policy
  expansion, and learning apply are deferred reopen gates, not phases in this
  optimization plan.

Exit criteria:

- Documentation no longer implies that `Decision`, `Finding`, or a new global
  `EvidenceRef` are required for this optimization.
- Documentation no longer implies that `schema_version` is required before a
  disk-layout migration exists.
- Deferred expansion items are clearly separated from current work.

## Deferred Reopen Gates

The following are not part of this optimization plan.

### Typed Host Contract

Reopen only when at least one real non-CLI host consumer needs a stable machine
contract, or CLI JSON compatibility pressure requires versioning.

At that point, design the contract from existing command behavior. Do not
prebuild `model.Decision` now.

### Global Finding Model

Reopen only when `ReasonCode` cannot express policy/runtime/review/health
diagnostics needed by a real consumer.

Any future model must be ReasonCode-compatible. Avoid severity names that fight
the current `info`, `warning`, and `error` severity values.

### Typed Evidence References

Reopen only when kernel-facing evidence needs typed metadata beyond the current
string map and lifecycle-event summaries.

Avoid a global type named `EvidenceRef` unless the existing
`internal/engine/status.EvidenceRef` conflict is resolved intentionally.

### Runtime Model Unification

Reopen only when wave/checkpoint/resume repair work shows repeated duplication
that cannot be fixed by local helpers.

Do not introduce `ExecutionRun` or `WorkItem` until that pressure exists.

### Policy Expansion

Reopen only when advisory policy packs need machine-readable diagnostics beyond
current bounded summaries.

Policy packs must remain unable to disable, downgrade, or override built-in
guardrails.

### Learning Apply

Reopen only when preview proposals are reviewed often enough to justify
proposal persistence, approval records, application records, and rollback.

`learn --preview` remains the product until then.

### Worktree And Orchestrator Promotion

Reopen only when all of the following are true:

- recovery semantics for concurrent execution are explicit and tested;
- workspace binding integrity checks are fail-closed;
- a concrete user-facing worktree command model exists;
- concurrent execution can be either fully supported or explicitly rejected;
- host mutation paths use the same lock/event/service path as CLI mutation;
- the deferment ADR and command contract matrix are updated in the same change.

Until then, orchestrator support remains hints and manual-only helper
workflows.

## Things To Prevent

Prevent these implementation shapes:

- speculative `Decision`, `Finding`, or evidence wrapper types without a real
  consumer;
- broad service extraction that only forwards to existing engine functions;
- treating `nextView` width as debt without a second consumer or JSON
  versioning pressure;
- changing public command semantics and internal state layout in one patch;
- adding `schema_version` before an actual disk-layout migration ADR;
- moving worktree binding out of `change.yaml` as a second state authority;
- policy packs that duplicate or weaken preset/control semantics;
- learning apply without proposal, approval, application record, and rollback;
- orchestrator code that writes Slipway files directly.

## Verification Strategy

For each implementation patch:

- run targeted tests for the touched command or package;
- run `go test ./... -count=1`;
- run `go vet ./...` when the change touches exported APIs or model helpers;
- inspect command JSON compatibility for `next`, `run`, and any touched command;
- confirm `change.yaml` serialized shape is unchanged unless a separate ADR
  explicitly authorizes a migration.

Static analysis is optional when available, but it is not a substitute for the
contract tests above.

## Residual Risks

- A helper-method pass can become abstraction churn if it tries to migrate every
  direct assignment. Keep Phase 1 limited to transition-heavy production paths.
- A future typed host contract may still be useful. Deferring it now avoids
  paying the parallel-model cost before a real consumer exists.
- Command-surface guards can become brittle if they encode help text instead of
  product command identity. Prefer command registry or root command membership
  assertions.
- Leaving `nextView` in `cmd/` is acceptable while it is a compatibility output
  shape. Revisit only when a second consumer needs a non-CLI contract.
