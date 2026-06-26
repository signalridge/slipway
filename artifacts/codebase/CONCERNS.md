# Concerns

- Route divergence risk: `status` currently succeeds from the root checkout for
  a single active change bound elsewhere, while `next`, `validate`, `done`, and
  `evidence` fail closed through `resolveActiveChangeRef`. A partial fix in one
  command would preserve misleading agent guidance.
- Local actionability risk: `next_ready_actions` currently describes lifecycle
  actions by state only (`cmd/common.go:994-1018`). It can therefore advertise
  `next`, `run`, or `done` in a workspace that cannot execute those actions
  locally.
- Archived-local regression risk: #283 behavior intentionally prefers an
  archived change in the current archived worktree over an unrelated active
  change elsewhere. A shared route abstraction must encode this as a first-class
  route kind instead of treating it as an afterthought.
- Error taxonomy drift risk: explicit missing slugs currently vary by command
  (`status --change` returns `change_not_found`, `validate --change` falls back
  to diagnostics, and `next --change` reports `no_active_change`). The shared
  route must pin stable error codes and exit code 3.
- Freshness overclaim risk: execution evidence freshness is not the same as
  governance readiness. Existing tests intentionally allow `required_skill_missing`
  while execution freshness remains `fresh`, so adding new readiness fields is
  safer than changing the execution-freshness helper semantics.
- Action-contract drift risk: `next` owns `confirmation_requirement` and action
  kinds, while `status` and `validate` expose separate recovery/action fields.
  `status`/`validate` must not let `recovery.primary_action` override or
  contradict the current `next` action kind.
- Capability dead-end risk: technique hints and generated skill prose can tell a
  host to spawn or delegate, but the CLI does not currently report whether that
  capability exists. Auto-mode must not report prior authorization sufficient
  when the selected skill requires an unavailable host capability and no explicit
  fallback is selected.
- Compatibility risk: new JSON fields should be additive where possible, while
  erroneous existing values that cause unsafe action claims should be corrected
  even if tests need updates.
