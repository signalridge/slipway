# Assurance

## Scope Summary

This change resolves GitHub issue #184 by making the generated
`wave-orchestration` surface executable for `parallel: true` waves through real
executor fan-out where the host runtime has a subagent or fresh-session
primitive.

Delivered scope:

- `wave-orchestration/SKILL.md.tmpl` now states that capable runtimes must use
  real executor subagent fan-out for `parallel: true`; same-context inline
  execution is not equivalent.
- `executor-dispatch-reference.md` maps Codex to `spawn_agent` semantics with
  deferred `tool_search` discovery, `fork_context: false`, collect/wait/parse
  and close semantics, and a fail-closed boundary for unavailable claimed
  isolation.
- Incapable runtimes remain distinguishable from capable-runtime failures and
  must report structured degraded sequential dispatch evidence.
- Executor results now have a stable contract: `task_id`, `verdict`,
  `changed_files`, `test_summary`, `evidence_ref`, `blockers`, and concise
  notes.
- Task evidence ownership remains with the coordinator through
  `slipway evidence task`; executors must not self-stamp governed freshness or
  write verification YAML.
- Single worktree parallelization is explicit: executor agents share the
  current worktree, so the coordinator must perform a target-overlap preflight
  before dispatch and a post-result changed-file conflict check before
  integration or the next wave.
- Wave dispatch evidence now requires a structured parallel dispatch mode and
  one per-task executor handle reference such as
  `executor_agent:wave=<wave_index>:task=<task_id>:<handle>`.
- Wait and recovery paths now fail closed for lost handles, missing parseable
  results, and stalled executor dispatch.
- Codex guidance now names the explicit user authorization boundary for
  `spawn_agent`: stop and ask when authorization is required but absent.
- Focused source and generated-output tests prevent regression to the old
  primary `codex -q --task` dispatch wording or unsafe single-worktree fan-out.

No Go runtime scheduler, persisted model schema, dependency, lockfile, public
command flag, or external API was changed.

## Verification Verdict

Current governed evidence verdict: pass.

Verified commands:

- `go test -count=1 ./internal/tmpl ./internal/toolgen`: failed as expected
  after adding the expanded single-worktree contract assertions, then passed
  after updating the template/reference implementation.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check`: pass,
  `docs/SURFACE-MANIFEST.json is up to date`.
- `git diff --check`: pass.
- `go test -count=1 ./...`: pass.
- `go run . validate --json`: fresh evidence and `scope_contract.status=pass`
  before assurance was authored; final active validation remains the next
  readiness proof after this assurance artifact exists.

Governance review verdicts:

- `spec-compliance-review`: pass with `layer:R0=pass`,
  `scope_contract:pass`, and `negative_path:pass`.
- `code-quality-review`: pass with `layer:IR1=pass` and
  `toolchain_compat:pass`.

## Evidence Index

- `verification/execution-summary.yaml`: refreshed wave evidence covers
  completed tasks `t-01` through `t-06`.
- `verification/wave-orchestration.yaml`: wave execution evidence recorded.
- `verification/spec-compliance-review.yaml`: bidirectional spec trace passed.
- `verification/code-quality-review.yaml`: implementation quality and
  generated-surface compatibility passed.
- `verification/spec-compliance-review-notes.md`: forward and reverse trace
  matrix for REQ-001 through REQ-006.
- `verification/code-quality-review-notes.md`: Stage 2 quality, test, safety,
  and toolchain review.

## Requirement Coverage

- REQ-001: covered by `SKILL.md.tmpl` fan-out wording and generated-surface
  assertions in `internal/toolgen/toolgen_test.go`.
- REQ-002: covered by the Codex `spawn_agent` adapter guidance and source plus
  generated-output tests for `spawn_agent`, `tool_search`,
  `fork_context: false`, collect/wait/parse/close semantics, and absence of the
  old primary `codex -q --task` path.
- REQ-003: covered by fail-closed capable-runtime wording in the template and
  reference, plus tests for capable-runtime and non-silent inline execution
  language.
- REQ-004: covered by structured degraded sequential dispatch wording and tests
  pinning `dispatch_mode:wave=<wave_index>:degraded_sequential`.
- REQ-005: covered by the stable executor result contract and evidence
  ownership wording, with tests for required fields and the
  `slipway evidence task` ledger boundary.
- REQ-006: covered by focused `internal/tmpl` and generated `internal/toolgen`
  tests, plus focused and full-suite verification.
- REQ-007: covered by single worktree target-overlap preflight wording in the
  template and dispatch reference, plus source and generated-output tests.
- REQ-008: covered by structured `dispatch_mode` and per-task
  `executor_agent:wave=<wave_index>:task=<task_id>:<handle>` evidence wording,
  plus source and generated-output tests.
- REQ-009: covered by lost-handle, missing-result, and stalled-dispatch
  recovery wording in the template/reference and tests.
- REQ-010: covered by Codex explicit user authorization fail-closed wording and
  generated Codex dispatch tests.

## Residual Risks and Exceptions

- No blocking residual risks.
- Workspace-local generated adapter copies are not tracked by this repository.
  Users who want refreshed local AI-tool surfaces after merge should run
  `slipway init --tools all --refresh`.
- Typed dispatch fields and executor-brief helpers remain deferred follow-ups
  by decision. The internal Go scheduler approach remains intentionally
  rejected for this issue.
- Per-executor git worktree isolation and GSD's commit protocol remain out of
  scope by user direction; this change implements single worktree
  parallelization safeguards instead.

## Rollback Readiness

Rollback is a normal git revert of the template, reference, test, codebase-map,
and governed artifact changes from this branch. After rollback, run:

```bash
go test -count=1 ./internal/tmpl ./internal/toolgen
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go test -count=1 ./...
```

No data migration, dependency downgrade, or external service rollback is
required.

## Archive Decision

Not archived. The requested stopping point is done-ready, not `slipway done`.
After this assurance artifact is present, capture a fresh active
`go run . validate --json` readiness proof before claiming done-ready. Do not
describe the archived bundle as revalidated through the active validate gate.
