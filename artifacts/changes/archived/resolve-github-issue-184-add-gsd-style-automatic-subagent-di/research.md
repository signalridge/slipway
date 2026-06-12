# Research

Question: how should Slipway borrow GSD-style executor fan-out for
`parallel: true` waves while preserving Slipway's generated-surface and
governance boundaries?

## Alternatives Considered

### Architecture

- Affected modules:
  `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`,
  `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md`,
  `internal/tmpl/*_test.go`, and `internal/toolgen/toolgen_test.go`.
- Dependency chain: template/reference sources -> `internal/toolgen.Generate`
  -> generated adapter skills and support files -> host runtime behavior during
  governed `S2_EXECUTE`.
- Blast radius: generated wave-orchestration instructions for all supported
  AI tools, focused tests under `internal/tmpl` and `internal/toolgen`, and no
  Go runtime scheduler behavior unless later scope expands.
- Constraints: the CLI owns lifecycle and evidence; task evidence still flows
  through `slipway evidence task`; `parallelization: off` remains an explicit
  sequential opt-out; Codex guidance cannot claim Claude worktree isolation;
  this issue now intentionally targets single worktree parallelization rather
  than per-executor git worktree isolation.

### Patterns

- Existing generated-surface contract tests already assert rendered
  wave-orchestration behavior in `internal/toolgen/toolgen_test.go:1069-1094`.
- Existing thin-host tests already read the executor dispatch reference directly
  in `internal/tmpl/thin_host_content_test.go:85-91`.
- Existing template convention is prose contract plus contract tests, not a new
  scheduler abstraction. This matches issue #184's minimum viable scope.
- GSD provides useful adapter semantics: orchestrator coordinates while
  subagents execute, runtime-specific spawning is required when available, Codex
  maps `Task(...)` and `Agent(...)` to `spawn_agent(...)`, `fork_context: false`
  keeps agent contexts fresh, and spawned agent IDs are collected, waited on,
  parsed, and closed.
- GSD's stronger worktree-isolation flow also highlights missing safeguards for
  a single shared worktree: pre-dispatch target-overlap checks, durable
  executor handle records, explicit wait/stall recovery, and post-result
  changed-file conflict checks.
- Slipway already has static wave target conflict logic in
  `internal/engine/wave/wave.go` and tests in
  `internal/engine/wave/wave_test.go`; the generated host contract should reuse
  that same fail-closed idea at dispatch time instead of inventing a parallel
  write lock.

### Risks

- High: leaving capable-runtime inline fallback as nonblocking would preserve
  the reported context-pollution failure.
- Medium: over-copying GSD worktree isolation would create a false Codex
  contract; local GSD tests document that Codex has no direct mapping for
  `Agent(... isolation="worktree")`.
- Medium: weak tests could pass while generated adapter output regresses; use
  both source/template tests and toolgen-generated output tests.
- Medium: single worktree fan-out without a host-side target preflight could
  let two executor agents edit overlapping files even when the planned wave no
  longer remains target-disjoint.
- Medium: a lost executor handle, missing parseable result, or stalled wait
  could make the coordinator continue with incomplete wave evidence unless the
  generated contract blocks recovery explicitly.
- Low: no new public surface row is expected, so `docs/SURFACE-MANIFEST.json`
  should remain unchanged unless implementation adds a surface.
- Guardrail domains: none detected.
- Reversibility: template/test edits are straightforward to revert and do not
  migrate persisted user data.

### Test Strategy

- Existing coverage: generated wave-orchestration high-level contract and
  reference path checks.
- New coverage needed:
  Codex `spawn_agent`; no primary `codex -q --task`; spawn/wait/collect/close;
  `fork_context: false`; coordinator stop-work while executors run; executor
  result fields `task_id`, `verdict`, `changed_files`, `test_summary`,
  `evidence_ref`, `blockers`; `slipway evidence task` as the evidence writer;
  no subagent self-stamping; no silent inline execution for capable runtimes;
  single worktree target-overlap preflight; executor handle evidence; wait,
  missing-result, and stalled-dispatch blockers; post-result changed-file
  conflict detection; Codex explicit authorization boundary.
- Verification commands:
  `go test -count=1 ./internal/tmpl ./internal/toolgen`,
  `go test -count=1 ./...`, `git diff --check`, and
  `go run . validate --json`.

### Options

- Option 1: Surface-contract fix only, with strong generated-surface tests.
  Update the wave-orchestration template/reference to name the runtime adapter
  contract, strengthen fallback semantics, and define the single worktree
  dispatch safety backstops. This matches the issue's implementation target and
  keeps the CLI as lifecycle authority.
- Option 2: Add typed dispatch-mode fields or runtime capability hints to CLI
  state. This could make review stricter, but it expands schema and runtime
  behavior beyond the minimum issue scope.
- Option 3: Build an internal scheduler/executor in Slipway. This would enforce
  behavior in code, but it contradicts the issue non-goal and significantly
  increases blast radius.
- Selected: Option 1. It directly closes the reported generated-surface gap,
  leaves typed dispatch metadata and helper commands as follow-up ideas, and
  gives tests enough specificity to prevent regression back to shell/prose-only
  Codex dispatch or unsafe same-worktree fan-out.

## Unknowns

- Resolved: Which exact GSD Codex adapter semantics should be borrowed into
  Slipway? -> Borrow orchestrator/coordinator separation, path-only executor
  prompts, runtime-specific subagent dispatch, `spawn_agent` mapping, deferred
  `tool_search` discovery, `fork_context: false`, agent ID collection, wait,
  structured parsing, close, and coordinator stop-work while executors are
  active.
- Resolved: Which GSD behavior must stay out of scope? -> Do not copy GSD
  project-state files, phase model, commit protocol, fail-soft skip paths, or
  Claude worktree isolation claims for Codex.
- Resolved: Is the target now single worktree parallelization or per-executor
  git worktrees? -> Single worktree parallelization. Borrow GSD's dispatch,
  handle collection, wait, and recovery ideas, but replace GSD worktree
  isolation with target-overlap preflight, executor-handle evidence, and
  post-result changed-file conflict checks in the shared current worktree.
- Resolved: How should Codex authorization limits be represented? -> If the
  current Codex runtime requires explicit user authorization for `spawn_agent`
  and the invocation lacks it, the generated guidance must ask for operator
  direction and record the boundary instead of forcing spawn or silently
  completing inline.
- Resolved: Which generated surfaces are tracked and must be regenerated? ->
  No project-local generated `.claude`, `.codex`, `.cursor`, `.gemini`, or
  `.opencode` adapter copies are tracked in this repository. Template sources
  and tests are tracked. `docs/SURFACE-MANIFEST.json` is regenerated only when a
  new public surface row is added; this change edits an existing skill surface.
- Resolved: Which tests should be expanded versus added? -> Expand
  `TestWaveOrchestrationSkillForcesParallelByDefault` for generated adapter
  output, and add focused `internal/tmpl` assertions for reference-level Codex
  adapter semantics and executor result contract.
- Remaining: None.

## Assumptions

- The issue body is the acceptance contract. Evidence: live GitHub issue #184
  body retrieved on 2026-06-12.
- Generated-surface tests should assert product text because the text is the
  runtime contract. Evidence: existing `internal/toolgen/toolgen_test.go`
  checks generated skill prose directly.
- The surface manifest should not change unless a new public surface row is
  introduced. Evidence: `docs/ai-tools.md:83-93` says the manifest inventories
  generated adapter, command, skill, JSON, and documentation surfaces.

## Canonical References

- `https://github.com/signalridge/slipway/issues/184`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:69`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:102`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:118`
- `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md:34`
- `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md:48`
- `internal/toolgen/toolgen_test.go:1069`
- `internal/tmpl/thin_host_content_test.go:56`
- `internal/tmpl/wave_isolation_content_test.go:11`
- `docs/ai-tools.md:83`
- `docs/operator-guide.md:139`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/gsd-core/workflows/execute-phase.md:12`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/gsd-core/workflows/execute-phase.md:24`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/gsd-core/workflows/execute-phase.md:492`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/gsd-core/workflows/execute-phase.md:558`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/gsd-core/workflows/execute-phase.md:687`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/bin/install.js:3439`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/bin/install.js:3450`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/bin/install.js:3460`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/tests/bug-279-codex-agent-mapping.test.cjs:12`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/tests/bug-3360-codex-execute-phase-worktrees.test.cjs:1`
- `internal/engine/wave/wave.go:196`
- `internal/engine/wave/wave_test.go:245`
