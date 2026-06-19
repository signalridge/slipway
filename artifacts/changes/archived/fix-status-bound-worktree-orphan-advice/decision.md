# Decision

## Alternatives Considered

### Option A: Filter orphan scans globally by same-slug authority
Change `state.OrphanBundleSlugs` so it suppresses an orphan result whenever any
workspace has an authority for the same slug.

Tradeoff: this would reduce false positives for issue #266, but it changes a
shared repair/health primitive that intentionally scans all worktrees. It risks
hiding real residue that `health`, `repair`, or root-level `status` should still
surface.

### Option B: Select any active change before delete recovery
Move unscoped status active-change route selection ahead of
`deleteRecoveryStatusView`.

Tradeoff: this is smaller in `cmd/status.go`, but it would suppress existing
root-level diagnostics for stale runtime bindings or unrelated orphaned bundles
whenever there is one active change. Existing delete recovery tests expect those
diagnostics to remain visible.

### Option C: Prefer only the current worktree's runtime binding
Add a state helper that resolves the active change bound to the current git
worktree directly from runtime `worktree-binding.yaml` records, without calling
`ListChanges`. Let unscoped status use that helper before global orphan/stale
diagnostics.

Tradeoff: this adds one narrow state API, but it avoids changing global orphan
semantics and only changes behavior when the invocation is inside a valid active
bound worktree.

## Selected Approach

Select Option C.

The fix is intentionally narrow: `slipway status --json` first asks whether the
current git worktree is bound to an active change via runtime binding. If yes,
status renders that active change. If no, if the binding points at corrupt or
missing authority, or if no matching active change exists, status falls back to
the existing `ListChanges` and delete-recovery path.

This satisfies REQ-001 without weakening REQ-002.

## Interfaces and Data Flow

- New state API: `state.FindActiveChangeByWorktreeBinding(root, currentWorktreePath)`.
- The API scans `.git/slipway/runtime/changes/*/worktree-binding.yaml`, matches
  the normalized binding path to the current git worktree, then loads the
  matching change through `state.LoadChange`.
- `cmd/status.go` calls this helper only in the unscoped no-`--change` branch,
  before `state.ListChanges` and `deleteRecoveryStatusView`.
- Public CLI flags, JSON field names, and explicit `--change` behavior are
  unchanged.

## Rollout and Rollback

Rollout:
- Ship the routing priority change with cmd-level and state-level regression
  tests.
- Verify focused tests first, then run broader package and repository tests.

Rollback:
- Revert the `cmd/status.go` call to `statusChangeFromCurrentWorktreeBinding`,
  remove `FindActiveChangeByWorktreeBinding`, and remove the two regression
  tests.
- Verification command for rollback or forward fix: `go test ./cmd ./internal/state`.

## Risk

- The main risk is hiding legitimate delete recovery. This is mitigated by
  leaving global orphan/stale logic unchanged and adding tests that the partial
  delete and stale runtime paths still route to delete recovery.
- A secondary risk is trusting stale runtime binding from another checkout. This
  is mitigated by reusing `readWorktreeBinding`, which ignores bindings written
  against a different git common directory.
- The change does not touch auth, credentials, PII, schema migrations, or
  external API contracts.
