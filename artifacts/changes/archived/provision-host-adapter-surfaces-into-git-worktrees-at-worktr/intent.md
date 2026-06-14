# Intent

## Summary
Provision host-adapter surfaces into git worktrees: at worktree creation (and idempotently on reuse/re-bind) copy each existing adapter dir (.claude/.cursor/.codex/.opencode/.gemini, excluding worktrees/, lock files, and generated sentinels) into the new worktree, then run toolgen.Generate(refresh) so slipway-owned skills/hooks/settings are regenerated from the worktree source. Fixes subagents in change worktrees losing hooks and being unable to read bundled skill references.

## Complexity Assessment
complex
<!-- Rationale: touches the worktree-provisioning kernel (internal/state/worktree.go) on both create and reuse paths, must generalize across all 5 toolgen adapters, combines a filesystem copy (third-party surfaces toolgen does not own) with a toolgen regeneration (slipway-owned surfaces), and adds fail-closed error handling plus tests. -->

## Guardrail Domains
<!-- none detected: infra/tooling surface, not auth/credentials/PII/financial/schema/external-API. -->

## In Scope
- `internal/state/worktree.go`: after a successful `git worktree add` (create path) and on the reuse/re-bind path, provision host-adapter surfaces into the worktree:
  - For each adapter dir present in the repo root among `.claude .cursor .codex .opencode .gemini`: copy into the worktree, EXCLUDING `<tool>/worktrees/`, lock files (e.g. `scheduled_tasks.lock`), and any generated sentinel (e.g. `.adapter-generated`). This carries the ~75 third-party skills toolgen does not own.
  - Then call `toolgen.Generate(worktreeRoot, toolgen.DetectExistingTools(repoRoot), refresh=true)` so slipway-owned skills/hooks/settings/commands are regenerated from the worktree's OWN source (not stale main-version copies).
  - Reuse/re-bind path is idempotent: copy only third-party dirs that are missing (do NOT clobber worktree-local third-party/manual edits), but ALWAYS regenerate `slipway-*` surfaces. This lazily backfills the ~60 already-existing under-provisioned worktrees on their next slipway bind.
  - Fail-closed: if copy or `toolgen.Generate` fails, return an error with actionable remediation; do not leave a half-provisioned worktree silently bound.
- Tests covering: a freshly created worktree contains `.<tool>/skills` with both a third-party skill dir and `slipway-*` dirs, plus hooks/settings; `slipway-*` reflect the worktree source; the reuse path backfills a missing surface without clobbering existing worktree-local edits; provisioning failure surfaces as a fail-closed error.

## Out of Scope
- `.serena/` — MCP cache that re-indexes itself; not provisioned/copied.
- A one-shot migration command to eagerly backfill all existing worktrees; existing worktrees are backfilled lazily on their next slipway bind, not swept proactively.
- Changing toolgen's generation logic itself (we only call its existing public API).

## Constraints
- Must generalize across all 5 toolgen adapters; no hardcoded `.claude`. Adapter set derived from `toolgen.DetectExistingTools` / registry, not a literal list duplicated in worktree.go where avoidable.
- Use existing toolgen public API only: `Generate(root, tools, refresh)`, `DetectExistingTools(root)`, `Registry()`.
- Must not recurse into `.<tool>/worktrees/` (Claude isolated-agent worktrees → recursion/bloat).
- Cost budget: ~2MB across 5 dirs per worktree — acceptable.

## Acceptance Signals
- New test passes: creating a worktree yields, for each detected tool, `.<tool>/skills` containing both a third-party skill dir and `slipway-*` dirs, plus hooks and settings.
- The `slipway-*` skills in the worktree reflect the worktree's own source (regenerated), provably distinct from a pure copy of stale main surfaces.
- Reuse-path test: a worktree missing a surface gets it on next bind, while an existing worktree-local third-party edit is preserved (not clobbered).
- Provisioning-failure test: an induced copy/generate failure makes worktree provisioning return a fail-closed error rather than binding a degraded worktree.
- `go build ./... && go vet ./... && go test ./...` green; `gofmt` clean.

## Open Questions
None.

## Deferred Ideas
- Optional `slipway worktree refresh` / migration command to eagerly re-provision all existing worktrees in one sweep (lazy backfill covers the need for now).

## Approved Summary
Provision host-adapter surfaces into git worktrees so subagents working in a change worktree get functioning hooks and readable bundled skill references. In `internal/state/worktree.go`, on both the create path (after `git worktree add`) and the reuse/re-bind path, copy each existing repo-root adapter dir among `.claude .cursor .codex .opencode .gemini` into the worktree — excluding `<tool>/worktrees/`, lock files, and `.adapter-generated` sentinels (this carries the ~75 third-party skills toolgen does not own) — then call `toolgen.Generate(worktreeRoot, DetectExistingTools(repoRoot), refresh=true)` to regenerate `slipway-*` skills/hooks/settings/commands from the worktree's own source.

Scope boundaries — In: both create and reuse paths; generalized across all 5 adapters; reuse is idempotent (copy only missing third-party dirs, never clobber worktree-local edits, always regenerate `slipway-*`), which lazily backfills existing under-provisioned worktrees on next bind; provisioning failure is fail-closed; plus tests. Out: `.serena/` (MCP cache, self-reindexing); no one-shot migration command to eagerly sweep all existing worktrees; no change to toolgen's generation logic itself.

Primary acceptance signal: a freshly created worktree contains, for each detected tool, `.<tool>/skills` with both a third-party skill dir and `slipway-*` dirs plus hooks/settings, with `slipway-*` reflecting the worktree source; reuse backfills without clobbering; provisioning failure returns a fail-closed error; `go build/vet/test ./...` green.

Confirmed by user on 2026-06-14.
