# Intent

## Summary
Repair the public lifecycle action contract described by `opt.md` section 1 as
one cohesive change. The goal is that `status`, `validate`, `next`, `run`,
`done`, and `evidence` route the same invocation consistently, report freshness
without overstating readiness, expose the same next-action contract, and fail
closed when host capabilities are insufficient.

## Complexity Assessment
complex
Rationale: this crosses multiple CLI public surfaces, shared change routing,
freshness/readiness semantics, generated/skill-facing copy, and black-box CLI
test fixtures. The surface is intentionally broader than a single resolver bug
because the user rejected the prior small-scope split and the failures are
coupled through the same action-contract model.

## Guardrail Domains
No sensitive-data, credential, financial, schema-migration, or external API
contract guardrail is in scope. This is public CLI lifecycle correctness and
agent-action safety.

## In Scope
- `opt.md` section 1.1 through 1.5 as a single lifecycle public-surface repair.
- Introduce a shared invocation route/actionability model, or an equivalent
  existing-pattern implementation, used by `status`, `validate`, `next`, `run`,
  `done`, and `evidence`.
- Route fields must identify invocation workspace, change authority path, bound
  workspace path, route kind, whether local lifecycle execution is allowed, and
  the effective remediation or next command.
- Align explicit missing `--change` handling across public commands, including
  `validate --change <missing>` returning `change_not_found` with exit 3.
- Split or supplement freshness/readiness JSON so execution evidence freshness
  cannot be mistaken for overall governance readiness.
- Align `status` and `validate` with `next` for the current action kind,
  especially S3 review-batch pending states.
- Add CLI-visible host-capability/delegation fail-closed behavior for the #339
  class of dead end, including fallback modes where supported.
- Preserve archived-local fallback behavior from #283.
- Add focused black-box tests and package tests for the route fields,
  `next_ready_actions`, remediation text, missing/archived/no-active cases,
  freshness combinations, review-batch action consistency, and host-capability
  permutations.

## Out of Scope
- `opt.md` section 2 release and supply-chain hardening.
- `opt.md` section 3 architecture dependency and coverage-gate hardening unless
  directly required by the lifecycle contract implementation.
- `opt.md` section 4 state-read performance baseline/indexing work.
- Reintroducing the reverted archive-only PR flow or splitting this change into
  narrow one-bug PRs.
- Cleaning unrelated existing worktrees or user dirt beyond the already reset
  and deleted abandoned changes.

## Constraints
- Use the current bound worktree as lifecycle authority.
- Preserve unrelated root untracked files: `.gemini/`, `coverage.out`, and
  `opt.md`.
- Keep changes scoped to public lifecycle/action-contract behavior and tests.
- Prefer existing command and view-building patterns over a new parallel
  framework.
- Work in auto-mode: the user's latest instruction authorizes best-judgment
  choices and blocker resolution without stopping for clarification.

## Acceptance Signals
- Root unscoped commands against a unique active change bound elsewhere do not
  advertise locally unexecutable `next/run/done` actions; they either fail with
  `change_bound_to_other_worktree` or provide executable remediation.
- Bound-worktree `status`, `next`, and `validate` expose consistent route fields.
- `validate --change definitely-not-a-change --json` exits 3 with
  `change_not_found`.
- Archived-local worktree status remains archived-local and does not regress
  #283.
- Freshness fields distinguish execution evidence, skill/governance evidence,
  and overall readiness so blocked ship gates cannot look completion-ready.
- S3 review-batch pending fixtures report the same action kind through `next`,
  `status`, and `validate`.
- Host capability/delegation unavailable states fail closed or expose an
  explicit supported fallback.
- Focused tests, `go test ./cmd -count=1`, `go test ./... -count=1`, and
  `golangci-lint run ./...` pass before ship.

## Open Questions
None.

## Deferred Ideas
- Release/ruleset/security hardening from `opt.md` section 2.
- Architecture dependency and coverage-gate work from `opt.md` section 3.
- State-read performance baseline/indexing work from `opt.md` section 4.

## Approved Summary
Approved via user auto-mode instruction on 2026-06-26 after resetting the
previous too-small attempt. This change repairs `opt.md` section 1 as a single
public lifecycle contract slice: shared route/actionability, explicit missing
change fail-closed handling, readiness-safe freshness fields, `status`/`validate`
alignment with `next`, and host-capability fail-closed behavior. Sections 2-4 of
`opt.md` are intentionally deferred to later changes.
