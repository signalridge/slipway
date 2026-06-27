# Decision

## Alternatives Considered

### Option A: copy engine context and wave logic into state
This would remove import violations quickly, but it would make
`internal/state` own more engine semantics and create duplicated scheduling and
freshness behavior. Rejected because it conflicts with REQ-002.

### Option B: introduce lower-level shared primitives and move engine-neutral
planning primitives out of `internal/engine`
Move generic freshness comparison into a lower package and move task-plan/wave
parse, hash, projection, and coverage primitives to a lower package shared by
engine, command, and state code. Keep lifecycle decisions in engine packages and
keep state focused on path resolution, strict artifact load/save, runtime
evidence I/O, and persisted cache metadata. Selected because it corrects the
dependency graph while preserving existing behavior and serialized artifact
schemas.

### Option C: add dependency injection around engine calls
This would keep public call sites smaller, but the production dependency would
remain easy to reintroduce and would add unnecessary abstraction around a package
boundary problem. Rejected because it does not provide a clear architecture
gate.

## Selected Approach

Use Option B.

Implementation will:
- add a lower-level freshness package for structural evidence freshness status
  and comparison;
- update `internal/state` freshness diagnostics to use that lower package
  instead of `internal/engine/context`;
- delete the old `internal/engine/wave` package, move its primitives to an
  engine-neutral lower package, and update all current Go consumers, including
  progression, governance, artifact validation, status views, scope contracts,
  command previews, state, and tests, without a compatibility shim;
- keep state responsible for persisted wave-plan and wave-run I/O, while
  lifecycle progression and review decisions remain in engine packages;
- extend `internal/architecture/dependency_direction_test.go` to forbid
  production `internal/state -> internal/engine` imports.

This keeps public serialized artifacts unchanged and makes the architecture
test enforce the intended boundary.

## Interfaces and Data Flow

- `internal/state` continues to expose persisted artifact I/O for execution
  summaries, wave plans, and wave evidence.
- Freshness status strings remain `fresh`, `stale`, and `unknown`.
- Wave-plan serialized data remains `model.WavePlan`; no YAML schema changes
  are introduced.
- Engine progression and command surfaces continue to call state for persisted
  artifact reads/writes.
- Engine, command, and state code import the same lower-level wave/task-plan
  primitive package for parse/hash/projection helpers, so state no longer
  imports `internal/engine` and there is still only one scheduling algorithm.

## Rollout and Rollback

Rollout is a normal code PR. Verification before merge:
- `rg -n 'github.com/signalridge/slipway/internal/engine|internal/engine/' internal/state`
- `go test ./internal/architecture ./internal/state ./internal/engine/context ./internal/engine/progression ./cmd -count=1`
- `go test ./internal/wave ./internal/freshness -count=1`
- `go test ./... -count=1`

Rollback is a normal revert of the PR. No data migration or external-state
rollback is required because artifact schemas are unchanged.

## Risk

- S1 to S2 progression depends on wave-plan materialization before implementation
  starts, so materialization behavior must be covered by targeted engine tests.
- Repair and health paths depend on current task-plan structural and scope hashes
  for stale evidence classification.
- Deleting `internal/engine/wave` requires all Go import consumers to move
  together; partial import updates would be a compile break or a scope escape.
- Freshness values are public lifecycle JSON/prose signals and must preserve
  exact string values.
- Architecture-gate changes must remain scoped to production imports and not
  block test-only integration fixtures.
