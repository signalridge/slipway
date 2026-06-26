# Decision

## Alternatives Considered

### Option A: Patch the root `status` path only

This would make `status --json` from the root checkout fail when the only active
change is bound to another worktree. It is small and would fix the most visible
bad output, but it would leave the rest of the public contract inconsistent:
`validate --change <missing>` would still return diagnostics with exit 0,
freshness would still conflate execution evidence with readiness, and `status`
and `validate` would still have their own action wording instead of matching
`next`.

### Option B: Shared route/actionability projection with additive public fields

This introduces one shared command-side projection for lifecycle invocation:
which workspace invoked the command, which change authority was selected,
whether the selected change is bound locally, whether local lifecycle execution
is allowed, and which remediation or command is executable next. `status`,
`next`, and `validate` expose that projection in JSON; mutation commands keep
using the same resolver so failures remain fail-closed. The same change adds
readiness-safe freshness fields and a shared action kind projection so `status`
and `validate` cannot disagree with `next`.

### Option C: Move route, readiness, and host capability into a new engine
service

This would be a cleaner architectural endpoint, but it would mix the lifecycle
contract repair with a larger internal refactor. It would also pull in parts of
`opt.md` section 3, which this change intentionally defers.

Selected: Option B. It is broad enough to repair `opt.md` section 1 as a single
contract, while keeping the implementation in the existing command/view
boundaries.

## Selected Approach

Implement a command-side invocation route/actionability contract and thread it
through the public lifecycle surfaces that currently diverge.

The route contract classifies at least these cases:
- locally bound active change
- active change bound to another worktree
- explicit missing change
- explicit archived/non-active change
- no active change
- ambiguous multiple active changes

For a locally bound active change, `status`, `next`, and `validate` report the
same route kind, invocation workspace, bound workspace, change authority path,
local executable flag, and executable next action/remediation.

For a root invocation against a unique active change bound elsewhere, public
commands must not advertise locally executable lifecycle actions. The acceptable
contract is either the existing fail-closed `change_bound_to_other_worktree`
precondition or a diagnostics view whose route fields clearly say local
execution is not allowed and name `cd <bound-worktree>` or `--change <slug>` as
the remediation.

For explicit missing changes, active lifecycle commands use `change_not_found`
with exit 3. `validate --change <missing> --json` stops returning successful
diagnostics.

Freshness output keeps the legacy `evidence_freshness` field as execution
evidence freshness, then adds explicit readiness-oriented fields:
- `execution_evidence_freshness`
- `governance_evidence_freshness`
- `overall_readiness_freshness`

`overall_readiness_freshness` is not allowed to be `fresh` when a required
governance gate is blocked by missing or stale skill evidence.

Action output adds a shared action kind projection derived from the same logic
as `next`. S3 review-batch pending states report `review_batch` through
`next --json --diagnostics`, `status --json`, and `validate --json`.

Host capability output is additive and narrow. It is attached only when the
selected host action actually requires delegation, subagent execution, or a
fresh independent review context. If that capability is unavailable and no
explicit fallback was selected, the contract fails closed instead of claiming
that prior authorization is sufficient. If a manual fallback is selected, the
view names the degraded mode and the evidence required to proceed.

## Interfaces and Data Flow

### Public JSON additions

`status`, `next`, and `validate` gain an additive `invocation_route` object with
fields equivalent to:
- `kind`
- `change_slug`
- `invocation_workspace_path`
- `bound_workspace_path`
- `change_authority_path`
- `local_lifecycle_execution_allowed`
- `remediation`
- `next_command`

`status` and `validate` gain additive action fields equivalent to:
- `current_action_kind`
- `current_action_command`

`status` and `validate` gain additive freshness fields:
- `execution_evidence_freshness`
- `governance_evidence_freshness`
- `overall_readiness_freshness`

`next`, `status`, and `validate` gain additive host capability information when
the selected action needs it:
- required capability
- availability
- fallback mode, when explicitly selected or supported
- evidence requirement for manual fallback

### Internal flow

Command entry points continue to resolve root and flags in `cmd`. The shared
route helper uses existing state functions and `resolveActiveChangeRef`
ingredients rather than introducing a separate state loader. Mutating commands
still fail before writes when the route is not locally executable.

`status` stops bypassing the resolver in the bound-elsewhere case. It either
returns the same fail-closed error as other commands or returns a diagnostics
view that removes locally unexecutable ready actions and carries explicit route
remediation.

`validate` stops swallowing explicit missing-change resolution errors into the
generic no-active diagnostics path. It only falls back to diagnostics for true
unscoped no-active or ambiguous contexts.

Freshness projection preserves the existing execution summary calculation, then
adds a readiness projection based on governance readiness blockers and required
skill evidence state.

Action projection reuses the next-view confirmation/action derivation where
possible and exposes the resulting action kind to `status` and `validate`.

## Rollout and Rollback

Rollout is a normal governed code change:
1. Add failing tests for the current public divergences.
2. Implement the shared route/actionability projection and route JSON.
3. Add readiness freshness and action kind fields.
4. Add narrow host capability fail-closed projection.
5. Run focused tests, `go test ./cmd -count=1`, `go test ./... -count=1`, and
   `golangci-lint run ./...`.
6. Finish the governed S3 review/final closeout flow before opening the PR.

Rollback is a git revert of the implementation commit or PR merge commit. The
verification command after rollback is `go test ./cmd -count=1`, followed by
`go test ./... -count=1` if the revert crosses package boundaries.

## Risk

The main behavioral risk is regressing archived-local status handling from
#283. The implementation keeps archived-local as a distinct route and adds a
regression assertion before changing route selection.

The second risk is overblocking normal auto-mode skill handoffs. Capability
fail-closed behavior is therefore limited to selected actions that actually
require unavailable delegation or independent-review capability; ordinary
non-sensitive handoffs keep the existing auto behavior.

The third risk is confusing existing JSON consumers by changing the meaning of
`evidence_freshness`. The implementation leaves that legacy field execution
oriented and adds clearly named readiness fields beside it.

The fourth risk is introducing another status-only projection. Tests will assert
that `status`, `next`, and `validate` expose the same route/action contract for
the same change state.
