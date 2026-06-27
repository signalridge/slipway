# Concerns

- Authority drift risk: an invocation-scoped read context must not outlive one
  command. It may reuse facts already read inside the command, but it must never
  become a persistent cache or durable index.
- Fail-closed risk: explicit `--change` fast paths must still preserve
  `change_not_found`, archived-change, sibling-bundle, missing-authority,
  bound-elsewhere, multi-active, and no-active semantics. Existing tests in
  `cmd/common_test.go`, `cmd/resolve_explicit_change_authority_test.go`, and
  `cmd/status_context_repair_test.go` are load-bearing.
- Hidden sibling risk: `internal/state` intentionally distinguishes visible
  workspace roots from hidden authority checks (`internal/state/store.go:170`,
  `internal/state/verification.go:131`). Fast paths must not skip hidden
  sibling checks where they are the reason a write/read fails closed.
- Verification drift risk: `status` currently calls the slug-based
  `ListVerifications` while readiness and next-skill views can use
  `ListVerificationsForChange`. Reusing resolved verification inventory should
  avoid duplicate reads but must keep strict YAML validation errors visible.
- Timeline risk: status displays only the last 20 events, but the existing
  reader validates the whole JSONL file. A tail reader improves performance but
  cannot silently skip malformed lines inside the retained tail. Full-log
  validation remains appropriate for health/repair surfaces.
- Performance fixture risk: generated 25+ worktree fixtures are intentionally
  bulky and must stay under `/tmp` or another ignored scratch area. Only the
  benchmark recipe and timing artifact belong in git.
