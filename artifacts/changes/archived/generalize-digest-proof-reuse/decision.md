# Decision

## Alternatives Considered
- Approach A: Extract a small reusable validator from the existing
  closeout-specific gate while keeping only the current closeout consumer. This
  is low risk but can become a superficial rename if no real edge abstraction is
  introduced.
- Approach B: Build a broad proof-type and stage-edge registry, including
  execution -> goal-verification reuse in the first pass. This has a stronger
  long-term shape but introduces too much public vocabulary and implementation
  surface before the second edge's constraints are proven.
- Approach C: Change only generated skill guidance so final-closeout is more
  reuse-first. This reduces rerun pressure but leaves the engine one-off intact.
- Approach D: Add an internal edge-spec validator and enable only the current
  closeout -> goal-verification edge in production. This gives the engine a real
  reusable primitive while keeping the first slice narrow.

## Selected Approach
Select Approach D.

The implementation will introduce a small internal proof-reuse edge/check shape
inside `internal/engine/progression`. The first production edge remains the
existing final-closeout reuse of goal-verification proof. Public evidence tokens
and blocker vocabulary remain unchanged:

- `closeout:goal_verification_reuse=pass`
- `closeout:goal_verification_reuse_run_version=<run_version>`
- `closeout_goal_verification_reuse_invalid`

The design deliberately does not expose a broad public registry in this slice
and does not change goal-verification's current role as producer of
`verification/suite-result.yaml`.

## Interfaces and Data Flow
- `final-closeout` records the existing closeout reuse references.
- Ship authority calls the compatibility wrapper for closeout reuse.
- The wrapper constructs an internal proof-reuse edge/check specification:
  source skill `goal-verification`, consumer skill `final-closeout`, current
  reuse run version, execution-summary freshness requirement, and digest checks
  for both source and consumer.
- The reusable validator checks:
  - source and consumer passing records exist where required
  - source, consumer, and execution-summary run versions agree
  - execution-summary is ready and fresh
  - source proof is not older than latest execution evidence
  - source and consumer skill digest inputs are fresh
- Failures are mapped through the existing closeout blocker factory so G_ship
  continues to expose `closeout_goal_verification_reuse_invalid`.
- Proof payload remains `SuiteResult`; no new proof artifact is introduced.

## Rollout and Rollback
Rollout is a normal code change in the governed worktree.

Verification commands:

```bash
go test ./internal/engine/wave ./internal/engine/progression -count=1
```

Rollback is a git revert of the implementation and generated-template changes.
Because the public closeout references and blocker names are preserved, rollback
does not require migration of existing governed evidence.

## Risk
- False reuse could skip required full-suite or SAST proof. Mitigation:
  preserve run-version equality, execution-summary freshness, suite-result
  digest inputs, changed-content digest inputs, and SAST digest checks.
- A too-small extraction could leave the one-off architecture intact. Mitigation:
  add tests for the internal edge-spec validator shape, not just the
  closeout-named wrapper.
- A too-large registry could destabilize lifecycle gates. Mitigation: keep the
  registry internal and enable only the existing closeout -> goal-verification
  edge in production.
- Template wording could imply an unsafe skip. Mitigation: generated guidance
  must say reuse is allowed only when engine-validated; rerun remains the
  fail-closed fallback.
