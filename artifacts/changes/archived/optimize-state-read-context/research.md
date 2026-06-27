# Research

## Alternatives Considered

### Architecture
- Affected modules: `cmd/status.go`, `cmd/status_view_build.go`,
  `cmd/next.go`, `cmd/next_context_build.go`, `cmd/validate.go`,
  `cmd/common.go`, `internal/state/store.go`, `internal/state/paths.go`,
  `internal/state/verification.go`, and
  `internal/state/lifecycle_event.go`.
- Dependency chains: CLI command constructors resolve root and route via
  `cmd/common.go`, load state through `internal/state`, then call
  `internal/engine/progression` for readiness. `internal/state` remains below
  `internal/engine`; this change must not add a reverse dependency.
- Blast radius: public read-only command views for `status`, `next`, and
  `validate`; state read helpers for bundle discovery, path resolution,
  verification inventory, and lifecycle JSONL tail reads.
- Constraints: `change.yaml` remains the lifecycle authority; git-local
  `worktree-binding.yaml` remains machine-local runtime authority; tracked
  `change.yaml` must not persist `worktree_path` (`internal/model/change.go:40`).

### Patterns
- Existing conventions: command entrypoints resolve a `changeRef` through
  `resolveActiveChangeRef` (`cmd/common.go:352`), then use command-specific
  builders. State path derivation is centralized in `ResolveChangePaths`
  (`internal/state/paths.go:29`).
- Reusable abstractions: `ListVerificationsForChange` already avoids
  rediscovering verification directory authority when a resolved change is
  available (`internal/state/verification.go:292`). `next` already passes
  `readiness.PassingSkills` into view assembly to avoid rereading verification
  files (`cmd/next_skill_view.go:830`).
- Convention deviations: introduce a small invocation-scoped read context that
  is passed through command builders. This is an explicit replacement at touched
  call sites, not a compatibility layer beside the old route API.

### Risks
- Technical risks: high risk if fast paths bypass hidden sibling/missing
  authority checks; medium risk if tail reads hide malformed JSONL in the
  displayed window; medium risk if performance helper tests overfit to private
  implementation details.
- Guardrail domains: no sensitive guardrail domain is touched; this is local
  filesystem/runtime state reading.
- Reversibility: the change is reversible by removing the invocation context and
  tail helper, but tests should pin fail-closed semantics so rollback does not
  silently reintroduce slow or misleading reads.

### Test Strategy
- Existing coverage: explicit missing slug, archived fallback, missing active
  authority, bound-worktree resolution, worktree-list cache invalidation,
  readiness optimization, and malformed lifecycle log health diagnostics are
  already covered in the files named in `artifacts/codebase/TESTING.md`.
- Infrastructure needs: add focused tests with injectable state/read counters
  where practical, plus command-level fixtures for `status`, `next`, and
  `validate` behavior. Keep bulky performance fixtures generated under `/tmp`.
- Verification approach: compare built-binary before/after timings with the same
  fixture shape; run targeted tests for changed packages; finish with
  `go test ./... -count=1`.

### Options
- Narrow command-local cleanup: remove the obvious duplicate `LoadChange` calls
  in `status`, `next`, and `validate`. Tradeoff: low risk, but it does not
  provide a shared route/read context, does not cover verification reuse, and
  does not satisfy `opt.md` 4.2.
- Invocation-scoped `StateReadContext`: create one per command invocation,
  resolve route/current change/paths once, pass it through `status`, `next`, and
  `validate`, reuse resolved verification inventory and timeline reads, and add
  explicit `--change` fast paths. Tradeoff: moderate call-site churn, but it
  directly matches `opt.md` 4.1-4.4 without persistent authority.
- Persistent workspace index: build or maintain a durable bundle/worktree index.
  Tradeoff: likely fastest for root summary, but explicitly out of scope and
  higher authority-drift risk.
- Selected: Invocation-scoped `StateReadContext`. It is the smallest approach
  that satisfies the performance and correctness requirements while honoring the
  user's no-compatibility-layer instruction and the current authority model.

## Unknowns
- Resolved: Duplicated read paths -> `LoadChange` is repeated after route
  resolution in `status`, `next`, and `validate`; `ListChanges` discovers slugs
  and calls slug-based loading; status evidence pointers use slug-based
  verification reads instead of the resolved-change helper.
- Resolved: Repeatable fixture -> generate a real temporary Git repository under
  `/tmp`, run `slipway init --tools none`, add at least 25 real Git worktrees,
  write 300 tracked-style `change.yaml` files, write git-local
  `worktree-binding.yaml` records, and place 100 verification records under the
  target change. The before-baseline artifact records the exact counts and
  timings.
- Resolved: Tail-read strategy -> add a read helper for status timeline windows
  that resolves the lifecycle event path once, reads from the file tail until it
  has enough non-empty lines, decodes retained lines with original line numbers,
  and fails closed on malformed retained lines. Full-log health/repair reads can
  continue using the existing complete decoder.
- Remaining: None for planning.

## Assumptions
- The current `/tmp` baseline fixture may be deleted by the OS; the durable
  artifact records the counts, command matrix, and measured timings needed to
  recreate it.
- Root unscoped status may remain slower than explicit single-change commands
  because it legitimately summarizes all visible active changes. Evidence:
  before-baseline root status is 3.29s against 300 changes.
- Tail-oriented status is allowed to validate only the retained display window;
  health/repair surfaces remain the place for complete lifecycle-log integrity
  scans. Evidence: `opt.md` 4.4 says status should not decode the full log just
  to show the last N events, while malformed logs still fail closed to
  diagnostics.

## Canonical References
- `/Users/yixianlu/ghq/github.com/signalridge/slipway/opt.md`
- `artifacts/changes/optimize-state-read-context/intent.md`
- `artifacts/changes/optimize-state-read-context/verification/state-read-baseline-before.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `cmd/common.go`
- `cmd/status.go`
- `cmd/status_view_build.go`
- `cmd/next.go`
- `cmd/next_context_build.go`
- `cmd/validate.go`
- `internal/state/store.go`
- `internal/state/paths.go`
- `internal/state/verification.go`
- `internal/state/lifecycle_event.go`
