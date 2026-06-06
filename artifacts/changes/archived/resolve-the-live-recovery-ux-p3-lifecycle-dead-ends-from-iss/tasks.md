# Tasks
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Worktree branch-mismatch rebind: route a bound-but-mismatched worktree through the existing worktree-preflight rebind by admitting the `dedicated_worktree_branch_mismatch` authenticity reason into `resolveS2Execute`, `DeriveWorktreeBlockers`, and the advance/readiness gates (relax the strict `WorktreePath == ""` precondition for that case only); retarget the `dedicated_worktree_branch_mismatch` recovery vocabulary from `slipway repair` to `slipway run`. worktree-preflight stays the sole `WorktreeBranch` writer; no git mutation.
  - wave: 1
  - depends_on: []
  - target_files: [internal/state/worktree.go, internal/state/worktree_test.go, internal/engine/progression/advance_governed.go, internal/model/recovery.go, internal/model/recovery_test.go]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: verdict
  - acceptance: A change bound to a worktree on a mismatched branch surfaces recovery `primary_command=slipway run`; advancing re-enters worktree-preflight and the recorded `WorktreeBranch` is reconciled to the worktree's actual branch with the mismatch blocker cleared; the recovery remediation no longer names `slipway repair`; preflight authenticity stays fail-closed for unregistered/non-dedicated worktrees.

- [x] `t-02` `slipway repair` actionability: dual-active finding names the conflicting slugs and `slipway status` / `slipway cancel --change <slug>` / `slipway done --change <slug>`; `repairDriftNextAction` default routes to a `slipway run` reopen instruction instead of "inspect the named artifact and rerun the owning Slipway command after correction".
  - wave: 1
  - depends_on: []
  - target_files: [cmd/repair.go, cmd/repair_test.go]
  - task_kind: code
  - covers: [REQ-002]
  - evidence: verdict
  - acceptance: Repair dual-active finding names the conflicting slugs plus an executable resolution command; the generic-drift default next action is a `slipway run` instruction; tests assert the executable commands and `NotContains` the old "inspect the named artifact and rerun" / bare "multiple active changes require operator intervention" dead-end strings.

- [x] `t-03` `slipway abort` repair-branch guidance: the `case "repair":` arm also names `slipway run` as the step that clears the interrupted-execution marker and continues, so abort→repair→status cannot loop.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/abort.go, cmd/abort_test.go]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: verdict
  - acceptance: After abort resolves to the repair branch, the printed guidance instructs `slipway repair` then `slipway run` to clear the interrupted-execution marker; a test asserts the repair-branch guidance contains `slipway run`; the common (resumable) path still advises `run`/`run --resume`.

- [x] `t-04` S2 scope guidance parity + docs/generated alignment: in `scopeContractNeedsRecoveryGuidance` drop the `state != S3 && state != S4` early-return so the scope-contract recovery guidance diagnostic is emitted at S2_EXECUTE too; update any docs/generated surfaces that describe the changed recovery vocabulary (branch-mismatch command, repair next actions, abort guidance) so they match the new behavior.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: [internal/engine/progression/readiness.go, internal/engine/progression/readiness_test.go, docs/commands.md, docs/operator-guide.md, internal/tmpl/templates, internal/toolgen]
  - task_kind: code
  - covers: [REQ-004, REQ-005]
  - evidence: verdict
  - acceptance: The scope-contract recovery guidance diagnostic is present at S2_EXECUTE (test-asserted) without changing the scope blockers/remediations; docs and generated surfaces describe the changed recovery vocabulary; `go run . init --refresh --tools all` leaves zero project-visible drift.

- [x] `t-05` Proof stack and dead-end replays: `go build ./...`, `go vet ./...`, `go test ./...`, `go test ./internal/toolgen/...`, `go run . init --refresh --tools all` + `git diff --check` (zero drift), and `go run . validate --json` green from the current worktree; replay each dead-end (worktree branch-mismatch → `slipway run`; repair dual-active/generic-drift actionable; abort→run clears interrupted execution; S2 scope guidance present) and confirm recovery JSON object field shape is unchanged.
  - wave: 3
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: [internal/toolgen]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: checklist
  - acceptance: All proof commands pass under the current worktree binary; the four dead-end replays each name an executable next action with no dead-end string; the public recovery JSON field shape is unchanged and generated surfaces show zero drift.
