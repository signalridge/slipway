# Decision

## Alternatives Considered

### Surface-contract update with contract tests

Update the `wave-orchestration` template and executor dispatch reference, then
pin the behavior with focused `internal/tmpl` tests and generated-output
`internal/toolgen` tests. This keeps the CLI as lifecycle/evidence authority
and directly fixes the product surface named by issue #184. The update includes
the single worktree dispatch contract: target-overlap preflight before spawn,
per-task executor handle evidence, bounded wait/stall recovery, and
post-result changed-file conflict checks.

### Typed runtime capability and dispatch-mode model

Add new typed fields to wave-orchestration verification or `next --json` so the
CLI can represent dispatch mode more structurally. This may be useful later, but
it expands schema behavior beyond the minimum issue and would need broader
consumer compatibility work.

### Internal scheduler/executor

Implement real executor orchestration inside Slipway's Go runtime. This would
enforce dispatch more strongly, but it contradicts the issue non-goal and would
turn a generated-surface contract fix into a large runtime feature.

## Selected Approach

Select the surface-contract update with contract tests.

This approach satisfies the explicit acceptance criteria: it makes
`parallel: true` executable for generated hosts, maps Codex to `spawn_agent`
semantics, prevents capable runtimes from silently executing inline, preserves
CLI-owned evidence, defines the shared-worktree safety checks, and adds tests so
the behavior cannot drift back to `codex -q --task` prose or untracked
single-worktree fan-out.

Typed dispatch fields, executor-brief helpers, and per-executor git worktrees
remain deferred follow-ups. The internal scheduler approach is rejected for this
issue.

## Interfaces and Data Flow

- Changed source interface:
  `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` changes the
  host-facing dispatch contract for S2 execution.
- Changed reference interface:
  `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md`
  changes the runtime adapter details consumed by generated host skills.
- Test flow:
  `internal/tmpl` tests render/read source templates and references directly;
  `internal/toolgen` tests generate adapter output in a temporary root and
  assert the shipped generated surface contains the same contract.
- Runtime flow remains:
  `slipway next --json` produces the wave plan; the host dispatches executors;
  the coordinator first checks that the current wave's declared targets remain
  disjoint in the shared worktree; executor handles are recorded per task;
  executor results return to the coordinator; missing results, lost handles, or
  stalled waits block for operator direction; the coordinator records task
  evidence through `slipway evidence task`; post-result changed-file conflicts
  and the post-wave integration gate run on the merged current worktree.

No Go command-line syntax, persisted model schema, or external API changes are
introduced.

## Rollout and Rollback

Rollout is the normal code path: commit template/reference/test changes. Users
refresh generated AI-tool surfaces with `slipway init --tools all --refresh`
when they want workspace-local adapters updated. This repository does not track
those generated adapter copies.

Rollback is a normal git revert of the template/reference/test changes, then:

```bash
go test -count=1 ./internal/tmpl ./internal/toolgen
```

If a new public generated surface is introduced during implementation, run:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
```

Current research indicates no manifest row change is expected.

## Risk

- Capable-runtime fallback must be worded carefully: unable-to-spawn on a
  capable runtime blocks or asks; genuinely incapable runtimes may report
  degraded/unsupported mode.
- Codex guidance must not overclaim worktree isolation. Local GSD explicitly
  treats `Agent(... isolation="worktree")` as having no direct Codex mapping.
- Single worktree fan-out must not imply write isolation. The generated host
  contract must fail closed on target overlap before dispatch and on returned
  changed-file conflicts before integration.
- Codex runtimes may require explicit user authorization for subagent spawning;
  the generated guidance must stop and ask instead of forcing spawn or silently
  inlining work.
- Tests must inspect generated output as well as template sources; otherwise
  toolgen regressions could pass source-only checks.
- `parallelization: off` and `parallel: false` must remain clean sequential
  paths with no degraded-mode noise.
