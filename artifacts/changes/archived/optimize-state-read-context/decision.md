# Decision

## Alternatives Considered
- Command-local duplicate removal: remove the second `LoadChange` calls in each
  command independently. This is low risk but misses verification reuse,
  timeline tail reads, and the shared context requirement.
- Invocation-scoped `StateReadContext`: create a per-command context that owns
  active route resolution, explicit fast-path loading, resolved paths,
  verification inventory, execution context, and status timeline reads. This
  satisfies the requirements without changing durable authority.
- Persistent workspace index: maintain a durable index of worktrees and bundles.
  This could improve root summary performance further but conflicts with the
  explicit no persistent authority/cache boundary.

## Selected Approach
Use an invocation-scoped `StateReadContext` and replace the touched command call
sites directly. The context is constructed at command entry, lives only for that
command, and is discarded before the process exits. It is not a compatibility
layer and not a durable index.

The explicit `--change` path will first try direct local authority and
git-local worktree binding authority, then fall back to the existing global
resolution path only for miss, archived, sibling, or integrity cases. This keeps
the normal success path fast while preserving existing fail-closed behavior.

## Interfaces and Data Flow
- Add a command-layer read context in `cmd/state_read_context.go`.
- Add or expose state helpers needed by the context:
  - direct active bundle load candidates for local and bound authority;
  - resolved-change verification inventory reuse;
  - lifecycle event tail reading for status display.
- Update `status`, `next`, and `validate` command entrypoints to construct one
  read context and pass it to their builders.
- Update readiness options only if needed to consume preloaded verification
  records without rereading the same files.
- `status` timeline rendering calls a tail-oriented reader for the display
  limit. Health and repair surfaces continue using the full lifecycle log reader
  for full-file integrity checks.

Data flow:

1. Command resolves project root and creates `StateReadContext`.
2. Context resolves route/change through explicit fast path or existing
   fallback semantics.
3. Builders reuse the resolved change, paths, execution context, verification
   records, and timeline tail where applicable.
4. Readiness and view rendering keep returning the existing JSON schema and
   typed error behavior.

## Rollout and Rollback
- Rollout is a normal code change behind no flags: tests pin the public behavior
  and before/after benchmark artifacts prove the performance effect.
- Rollback is `git revert` of the implementation commit plus rerunning targeted
  command/state tests. Because there is no persisted index or schema migration,
  rollback does not require data migration.
- Verification commands: targeted `go test` packages for touched areas, then
  `go test ./... -count=1`; built-binary after-baseline using the same fixture
  recipe recorded in the before-baseline artifact.

## Risk
- Fast-path risk: skipping global scans could hide sibling or missing-authority
  diagnostics. Mitigation: fast path falls back to existing global resolution
  when direct local/bound authority is absent or invalid.
- Stale cache risk: a read context could accidentally become process-global.
  Mitigation: construct it per command and keep fields unexported to `cmd`.
- Timeline risk: tail reads could skip malformed earlier lines. Mitigation:
  status validates the retained display tail, while health/repair remain the
  full-log integrity surfaces.
- Test fragility risk: read-count tests can overfit. Mitigation: use them only
  for the targeted fast/reuse contract and keep public behavior tests as the
  primary regression guard.
