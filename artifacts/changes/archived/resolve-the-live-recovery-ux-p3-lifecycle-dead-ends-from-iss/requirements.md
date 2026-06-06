# Requirements
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Worktree branch-mismatch reopens to an executable rebind
REQ-001: When a governed change is bound to a worktree whose actual git branch no longer matches the recorded `WorktreeBranch`, the recovery surface MUST name `slipway run` as the primary command (not `slipway repair`), and `slipway run` MUST re-enter worktree-preflight and reconcile the recorded branch to the worktree's actual branch, with no `git checkout`/HEAD mutation and no second writer of `WorktreeBranch`.

#### Scenario: Branch mismatch routes to slipway run and rebinds
GIVEN an active change at S2_EXECUTE bound to a dedicated worktree whose recorded `WorktreeBranch` differs from the worktree's current git branch
WHEN `slipway next`/`validate` is evaluated and then `slipway run` is invoked
THEN the recovery `primary_command` is `slipway run`, the change re-enters worktree-preflight, and after rebinding the recorded `WorktreeBranch` equals the worktree's actual branch and the `dedicated_worktree_branch_mismatch` blocker is cleared.

#### Scenario: Repair no longer claims to fix branch mismatch
GIVEN a `dedicated_worktree_branch_mismatch` blocker
WHEN the recovery remediation is rendered
THEN it does not point to `slipway repair` (which has no branch-rebind code path).

### Requirement: `slipway repair` findings name an executable next action
REQ-002: `slipway repair` MUST NOT emit non-actionable findings. The dual-active finding MUST name the conflicting change slugs and an executable resolution command (`slipway status`, and `slipway cancel --change <slug>` / `slipway done --change <slug>`); the generic drift finding's next action MUST be `slipway run` (reopen the earliest affected authority), never "inspect the named artifact and rerun the owning Slipway command after correction".

#### Scenario: Dual-active names slugs and commands
GIVEN repair detects more than one active change
WHEN the dual-active finding is rendered
THEN it names the conflicting slugs and an executable command to resolve them, and does not emit the bare "multiple active changes require operator intervention" with a generic next action.

#### Scenario: Generic drift routes to slipway run
GIVEN a repair drift finding that matches no specific keyword case
WHEN `repairDriftNextAction` computes the next action
THEN it returns a `slipway run` instruction, not "inspect the named artifact and rerun".

### Requirement: `slipway abort` names the interrupted-execution clearer
REQ-003: After `slipway abort` sets `InterruptedExecutionAt`, the printed guidance — including the branch that mentions `slipway repair` — MUST name `slipway run` as the step that clears the interrupted-execution marker, so the operator cannot loop repair↔status without ever clearing it.

#### Scenario: Abort repair branch names slipway run
GIVEN `slipway abort` resolves its next action to the repair branch (broken execution state)
WHEN the guidance text is printed
THEN it instructs the operator to run `slipway repair` to restore integrity and then `slipway run` to clear the interrupted-execution marker and continue.

### Requirement: Scope-contract recovery guidance has surface parity at S2
REQ-004: The scope-contract recovery guidance diagnostic MUST be emitted at S2_EXECUTE as well as S3_REVIEW/S4_VERIFY, so an operator inspecting an S2 scope-contract failure sees the same explanation that S3/S4 already show. The executable next action at S2 is already covered by per-blocker remediation and the scope-contract advance-reopen gate; this requirement is surface/narrative parity.

#### Scenario: S2 surfaces the scope guidance diagnostic
GIVEN a change at S2_EXECUTE with a scope-contract blocker
WHEN readiness diagnostics are computed
THEN the scope-contract recovery guidance diagnostic is present, as it already is at S3/S4.

### Requirement: Public recovery contract preserved and tested
REQ-005: The public recovery JSON object field shape (`primary_command`, `primary_action`, `recovery_class`, `steps[]`) MUST remain stable. Each intentional vocabulary change (the branch-mismatch command, repair next actions, abort guidance) MUST be covered by a contract/regression test, and docs plus generated surfaces MUST agree with the resulting behavior after `go run . init --refresh --tools all`.

#### Scenario: Recovery JSON shape unchanged with tested vocabulary
GIVEN the changed recovery/repair/abort surfaces
WHEN the recovery JSON and CLI text are rendered and the generated surfaces are refreshed
THEN the documented recovery object fields are unchanged, the intentional vocabulary changes are asserted by tests, and `git diff --check` after the refresh shows zero project-visible drift.
