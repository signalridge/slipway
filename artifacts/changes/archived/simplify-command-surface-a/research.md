# Research

## Alternatives Considered

### Architecture
- Affected modules: root command grouping and registration in `cmd/root.go`;
  checkpoint command/state paths in `cmd/checkpoint.go`, `cmd/run.go`,
  `cmd/stage.go`, `cmd/next.go`, `cmd/next_context_build.go`,
  `cmd/status_view_build.go`, `cmd/health.go`, `cmd/repair.go`, and
  `cmd/abort.go`; model and reason-code definitions in `internal/model`; wave
  task metadata in `internal/engine/wave`; generated command/docs surfaces in
  `internal/toolgen`, `internal/tmpl`, and `docs`. Evidence:
  `artifacts/codebase/ARCHITECTURE.md`.
- Dependency chains: `internal/toolgen.CommandDescription` feeds CLI help via
  `cmd/root.go`; toolgen command metadata also feeds docs, manifest, and
  generated skill surfaces. Checkpoint state flows from `model.Change` into
  `run`/stage entry validation, `next` handoff JSON, confirmation requirements,
  status recovery actions, health/repair diagnostics, lifecycle events, and
  templates. Evidence: `cmd/root.go:13-15`, `cmd/root.go:28-90`,
  `cmd/root.go:152-189`, `cmd/run.go:202-291`, `cmd/next.go:190-206`,
  `cmd/next.go:539-575`, `cmd/next.go:730-768`.
- Blast radius: broad but bounded to product and generated agent surfaces for
  Workstream A. The change removes command/state surfaces for `checkpoint`,
  `learn`, and `stats`; it must preserve retained lifecycle commands, task
  evidence, `run --resume`, `status --stats`, `health`, and fail-closed
  evidence checks.
- Constraints: do not change Workstream B result-import behavior, do not
  redesign Workstream C evidence wording except direct A leftovers, do not
  hand-edit generated verification YAML or runtime evidence state, and do not
  weaken freshness/digest gates.

### Patterns
- Existing conventions: command descriptions and argument metadata are owned by
  `internal/toolgen`, while root help renders grouped commands from `cmd/root.go`.
  Generated skill prose is authored in `internal/tmpl/templates`, not checked-in
  generated output. Evidence: `artifacts/codebase/STRUCTURE.md`,
  `artifacts/codebase/CONVENTIONS.md`.
- Reusable abstractions: retain `state.CollectRepoStats` if `status --stats`,
  `health`, or other retained diagnostics still use it. `cmd/status.go` already
  exposes `--stats`, and `internal/state/stats.go` owns reusable repo stats
  collection. Evidence: `cmd/status.go:150-175`, `cmd/status.go:330-337`,
  `internal/state/stats.go:43-70`.
- Convention deviations: deleting `ActiveCheckpoint` intentionally drops
  compatibility for active checkpoint state. This is a deliberate product
  simplification from issue #297 rather than a hidden migration.

### Risks
- Technical risks: high risk of incomplete deletion because checkpoint is
  coupled into `run`, stage drivers, `next`, status, health/repair, model
  validation, wave metadata, templates, docs, and tests. Medium risk that stale
  docs or generated manifest rows keep deleted commands visible. Medium risk
  that replacing checkpoint with blocked/incomplete task verdicts exposes a
  current `run --resume` gap.
- Guardrail domains: none detected by intake. The change touches workflow
  authority, so evidence and freshness gates still need fail-closed tests.
- Reversibility: command/docs/template deletions are reversible; model-state
  deletion is less reversible for active changes that contain
  `active_checkpoint`, but issue #297 explicitly accepts deleting the concept.

### Test Strategy
- Existing coverage: targeted `go test ./cmd -run
  'TestRunRequiresExplicitResumeAfterAbortWithWaveBackedState|TestRunRejectsResumeWhenWaveRunsAreIncomplete|TestRunResumeUnavailableExplainsLifecycleBoundary'`
  passes before changes, proving the non-checkpoint `run --resume` seam exists.
- Infrastructure needs: remove or replace checkpoint-specific tests; move any
  retained learn/stats helper coverage to retained owners; update toolgen,
  template, and manifest contract tests; add black-box help/search tests for
  deleted surfaces.
- Verification approach: run targeted packages for command/model/state/wave/
  progression/template/toolgen changes, regenerate or check the surface manifest,
  run black-box help checks, run the issue search checks, then run `go test ./...`.

### Options
- Option 1: Hide commands only. This is low-effort but rejected because the
  current facts already show `stats` is Cobra-hidden yet still exported through
  custom help/toolgen/docs, and it leaves `ActiveCheckpoint`,
  `--resume-response`, `resume_checkpoint`, and task metadata alive.
- Option 2: Direct Workstream A deletion with preserved ledger-backed resume.
  Delete `checkpoint`, `learn`, and `stats` from CLI/toolgen/docs/generated
  surfaces; remove checkpoint model/protocol branches; preserve `run --resume`,
  interrupted wave recovery, task verdict blockers, `status --stats`, and
  reusable internal stats. This best matches issue #297 A and keeps scope clear.
- Option 3: Add Workstream B result import first, then delete A surfaces. This
  follows the one-PR ordering suggested by the full issue, but it is rejected for
  this governed change because the approved scope is A only and B includes a
  larger engine-owned run-version boundary.
- Selected: Option 2. It is the smallest direction that satisfies Workstream A
  without preserving the old checkpoint protocol or drifting into B/C.

## Unknowns
- Resolved: Is the existing codebase map relevant? -> Partially no. Older map
  sections described an adapter expansion and were re-authored for issue #297 A
  before use. Evidence: `artifacts/codebase/ARCHITECTURE.md`,
  `artifacts/codebase/TESTING.md`, `artifacts/codebase/CONCERNS.md`.
- Resolved: Is `stats` already gone because Cobra marks it hidden? -> No.
  `cmd/stats.go` sets `Hidden: true`, but root help, toolgen metadata, docs,
  and manifest still expose `stats`. Evidence: `cmd/stats.go:37-42`,
  `cmd/root.go:74-80`, `internal/toolgen/toolgen.go:347-352`.
- Resolved: Does `learn` have an apply path worth preserving? -> No. The
  command returns `learn_apply_unsupported` whenever preview is false. Evidence:
  `cmd/learn.go:83-113`.
- Resolved: Does `run --resume` exist independently of checkpoint? -> Yes, it
  is keyed to S2 execution readiness and an incomplete wave index. Evidence:
  `cmd/common.go:930-950`, `cmd/run.go:270-289`.
- Remaining: None for planning Workstream A. Implementation must still prove
  whether blocked/incomplete task evidence gives a practical resume path after
  checkpoint deletion; if not, add the focused fix inside A.

## Assumptions
- Workstream A scope is authoritative for this governed change; Workstreams B
  and C remain out of scope except direct A-surface fallout. Evidence:
  `artifacts/changes/simplify-command-surface-a/intent.md`.
- The user authorized best available decisions on blockers and later changed
  native built-in subagent concurrency from 3 to 2; implementation will not
  depend on external agent runtimes. Evidence: current thread objective and
  user correction.
- No sensitive guardrail domain is in scope. Evidence:
  `artifacts/changes/simplify-command-surface-a/change.yaml`.

## Canonical References
- `https://github.com/signalridge/slipway/issues/297`
- `artifacts/changes/simplify-command-surface-a/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `cmd/root.go`
- `cmd/run.go`
- `cmd/stage.go`
- `cmd/next.go`
- `cmd/next_context_build.go`
- `cmd/status_view_build.go`
- `cmd/learn.go`
- `cmd/stats.go`
- `internal/model/change.go`
- `internal/model/reason_code.go`
- `internal/engine/wave/wave.go`
- `internal/engine/wave/parse.go`
- `internal/engine/progression/wave_sync.go`
- `internal/toolgen/toolgen.go`
- `internal/toolgen/install_profiles.go`
