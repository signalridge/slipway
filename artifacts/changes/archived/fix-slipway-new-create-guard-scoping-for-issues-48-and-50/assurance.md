# Assurance

## Project Context
- Tech Stack: Go CLI
- Conventions: command-level behavior in `cmd/`; state/worktree authority in
  `internal/state`; governed verification artifacts under this change bundle
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
GitHub issue #48 and issue #50 were both confirmed against the live code and
fixed in the `slipway new` create guard. The guard no longer serializes every
active change repo-wide; it rejects only when an existing active change owns the
current invocation scope workspace or the prospective target workspace for the
new change.

The implementation is intentionally narrow:
- `cmd/new.go` now constructs the prospective `model.Change` before the create
  guard runs, so the guard can inspect the resolved slug and `NeedsDiscovery`.
- `cmd/common.go` computes invocation-scope and prospective-target workspace
  authority before rejecting.
- `cmd/new_test.go` covers #48, #50, preserved unbound same-workspace rejection,
  preserved bound-current-worktree rejection, and slug namespace corruption
  diagnostics after the guard reorder.

No storage migration, new command surface, external API change, dependency
change, or #46-style bind/park workflow was added.

## Verification Verdict
Pass. Fresh verification after the final implementation change produced:
- `go test ./cmd -run 'TestNewCommand(RejectsWhenActiveChangeAlreadyExists|AllowsDiscoveryChangeWhenUnboundIntakeChangeOwnsRoot|AllowsBoundSiblingWorktreeActiveChange|RejectsWhenActiveChangeIsBoundToCurrentWorktree|FailsClosedWhenSlugNamespaceIsCorrupt)' -count=1`: pass.
- `go test -count=1 ./cmd`: pass.
- `go test -count=1 ./...`: pass for all packages.
- `go build ./...`: pass.
- `git diff --check`: pass.
- `go test -count=1 -coverprofile=/tmp/slipway-issue48-50-cmd.cover ./cmd`: pass, 80.2% statement coverage for `cmd`.
- `go run . validate --json`: active-change evidence freshness `fresh`,
  `skills_ready.goal-verification=pass`, `scope_contract.status=pass`, and only
  final-closeout/attestation blockers remaining before this final closeout.

The stub/placeholder scan found only ordinary helper returns in existing code
paths, not TODOs, placeholder bodies, empty implementation blocks, or hardcoded
production responses.

## Evidence Index
- `requirements.md`: five requirements covering #48, #50, collision semantics,
  prospective workspace targeting, and lifecycle evidence.
- `decision.md`: selected scoped workspace-collision guard approach.
- `tasks.md`: five completed execution tasks.
- `verification/execution-summary.yaml`: run version 1 with all five tasks
  passing and fresh runtime task evidence.
- `verification/wave-orchestration.yaml`: wave execution evidence.
- `verification/spec-compliance-review.yaml`: spec-to-code and code-to-spec
  trace.
- `verification/code-quality-review.yaml`: review evidence for localized
  implementation, safety, and test quality.
- `verification/goal-verification.yaml`: AC-level Exists/Substantive/Wired
  proof for both issues and preserved negative paths.
- `cmd/common.go`, `cmd/new.go`, `cmd/new_test.go`: implementation and
  regression coverage.
- `artifacts/codebase/`: refreshed source-backed repository context used during
  planning and review.

## Requirement Coverage
- REQ-001: Covered by
  `TestNewCommandAllowsDiscoveryChangeWhenUnboundIntakeChangeOwnsRoot`; an
  unbound intake-stage root-owned change no longer blocks a discovery follow-up
  that targets a default worktree.
- REQ-002: Covered by `TestNewCommandRejectsWhenActiveChangeAlreadyExists`; a
  root-owned unbound follow-up collision still fails closed with
  `active_change_exists` and does not suggest `slipway next`.
- REQ-003: Covered by `TestNewCommandAllowsBoundSiblingWorktreeActiveChange`;
  a hidden active sibling worktree authority does not block a root-scoped new
  discovery change with a different target workspace.
- REQ-004: Covered by `cmd/new.go` guard reorder,
  `createGuardInvocationWorkspaceRoot`, `newChangeTargetWorkspaceRoot`, and
  `TestNewCommandRejectsWhenActiveChangeIsBoundToCurrentWorktree`; current and
  prospective workspace authority are the collision boundaries.
- REQ-005: Covered by fresh targeted tests, full suite, build, diff check,
  command coverage, scope validation, and this governed closeout bundle.

## Residual Risks and Exceptions
- `targetWorkspaceCreateConflict` remains a defensive branch for an already
  owned prospective target workspace. The normal slug generation path makes
  that collision uncommon; the branch is retained so future custom target or
  slug-collision behavior fails closed instead of allowing double ownership.
- An unbound active change still resolves to the invocation root supplied to
  `WorkspaceRootForChange`. A non-discovery `slipway new` from another
  worktree can therefore still fail closed against an unbound root-owned
  change, with wording that may describe the current workspace imprecisely. This
  preserves the old blocking behavior and is outside the #48/#50 scope because
  the fixed paths are root-owned unbound intake plus discovery follow-up and
  bound sibling worktree isolation.
- `go run . repair --json` was tried while refreshing evidence and failed on an
  unrelated legacy `worktree_path` YAML field encountered by broad repair
  scanning. The governed change did not require repair after
  `execution-summary.yaml` was refreshed from runtime task evidence; subsequent
  `status --json` and `validate --json` reported evidence freshness as `fresh`.

No guardrail domain is active for this change, and no sensitive data,
credentials, destructive operations, schema migrations, or external contracts
were modified.

## Rollback Readiness
Rollback is local and low-risk: revert `cmd/common.go`, `cmd/new.go`, and
`cmd/new_test.go`, then rerun the targeted command regression set plus
`go test -count=1 ./...` and `go build ./...`. There is no data migration or
persisted user-state migration to unwind. Governed artifacts can be archived or
dropped with the branch if the code rollback is selected.

## Archive Decision
Archive-ready after final-closeout evidence is recorded and the active
`validate --json` gate is rerun before `slipway done`. The current pre-closeout
active validation already proves fresh execution evidence, passing
goal-verification, valid requirements, and a passing scope contract; the only
remaining blockers are the expected standard-preset final-closeout attestation
requirements.
