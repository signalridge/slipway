# Decision

## Alternatives Considered

### Option A: Documentation-only clarification
Leave runtime behavior unchanged and update documentation to say that auto only
softens confirmation text and preset confirmation. This has the lowest runtime
risk but does not fix the observed usability issue: after one advance, `run`
still often stops at a `run_slipway_run_to_advance` boundary and asks for the
same command again.

### Option B: Embedded full autopilot
Add a Slipway-managed executor that dispatches skills/subagents, runs tests,
records evidence, and finalizes done-ready changes. This would be broader than
the current CLI authority model and would blur the evidence boundary owned by
`slipway evidence skill`.

### Option C: Bounded auto-to-next-real-gate
Keep Slipway as lifecycle authority, but let auto continue through routine
command-required pass-through boundaries in the same run-loop invocation.
Continue to stop at skill handoffs and hard gates where an external host,
fresh evidence, or explicit finalization is required.

Selected: Option C.

## Selected Approach

Implement a narrow auto continuation predicate in the run/stage loop stop
logic. Under auto, if the current view's blockers are only the routine
`run_slipway_run_to_advance` command boundary (optionally paired with
`no_skill_required`) and there is no `next_skill` or review batch requiring host
execution, the loop continues. Otherwise the stop behavior remains unchanged.

This approach directly addresses the pass-through friction without pretending
that Slipway can execute governance skills or synthesize evidence. It uses the
existing boundary model instead of adding a second lifecycle interpretation.

## Interfaces and Data Flow

- `slipway run --auto` passes the effective auto setting into
  `runGovernedLoop`.
- `runGovernedLoopWithBuilder` will decide whether to stop using the current
  `nextView` plus the auto flag.
- `slipway intake` / `slipway plan` / `slipway implement` already read
  config-level `execution.auto`; their stage loop uses the same stop predicate
  while still stopping when the state leaves the stage.
- `slipway next` remains read-only and does not mutate state.
- `slipway evidence skill` remains the only path for skill evidence.

No durable state schema changes are required.

## Rollout and Rollback

Rollout is a normal code/test/doc change. Verification:

```bash
go test ./cmd -run 'Test.*Auto.*|TestRun.*'
go test ./internal/engine/progression -run 'Test.*Auto.*|Test.*Boundary.*'
```

Rollback is reverting the code/tests/docs in this change. Since no schema or
runtime state format changes are introduced, rollback is a source revert plus
the same focused tests.

## Risk

- The main risk is accidentally crossing a real hard gate. Mitigation: the
  continuation predicate only accepts routine command-required blockers and
  rejects any next skill, review batch, done-ready, unknown/noop, or non-pacing
  blocker.
- A secondary risk is infinite looping. Mitigation: the existing
  `maxAutoNextIterations` cap remains in place, and continuation is allowed
  only after an advancing command boundary.
- Documentation drift is possible because command surfaces are generated.
  Mitigation: update README/reference docs and toolgen strings/tests together
  when behavior changes.
