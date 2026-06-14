# Tasks

## Task List

- [x] `t-01` Implement the worktree host-surface provisioning helper in the surface-renderer layer: an exported `toolgen.ProvisionWorktreeHostSurfaces(repoRoot, worktreeRoot)` that iterates `toolgen.Registry()`, and for each adapter whose `ToolRootPath(cfg)` dir exists under repoRoot, recursively copies that tree into the worktree (skip-if-exists per file so worktree-local edits are never clobbered; exclude the `<toolroot>/worktrees` subtree, `*.lock` files, and the `.adapter-generated` sentinel), then calls `toolgen.Generate(worktreeRoot, presentToolIDs, refresh=true)`. Any copy or Generate error is returned to the caller (fail-closed). It lives in toolgen (not internal/state) so the authority layer never imports a surface renderer.
  - depends_on: []
  - target_files: [internal/toolgen/worktree_provision.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005, REQ-006, REQ-007]

- [x] `t-02` Wire provisioning into `EnsureDefaultWorktreeForChange` via dependency injection: define a `state.WorktreeProvisioner` function type (nil-safe) so `internal/state` declares the seam without importing toolgen; call the injected provisioner on the create branch (after `git worktree add` succeeds / `invalidateWorktreeListCache`, before returning `Created:true`) and on the reuse branch (the already-registered block, after worktree validation passes, before its return), propagating provisioning errors so binding fails closed; and pass `toolgen.ProvisionWorktreeHostSurfaces` from the `cmd/new.go` composition root.
  - depends_on: [t-01]
  - target_files: [internal/state/worktree.go, internal/state/worktree_provision.go, cmd/new.go]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-005, REQ-007]

- [x] `t-03` Author tests proving the provisioning contract. Unit-test `toolgen.ProvisionWorktreeHostSurfaces` directly: a provisioned worktree gains, for each detected tool, `.<tool>/skills` containing both a copied third-party skill dir and regenerated `slipway-*` dirs plus hooks/settings; the `worktrees/` subtree and `*.lock` files are absent, the source sentinel is not carried verbatim, and no `.serena/` is produced; a stale slipway-* skill in the source is overwritten by regeneration; a worktree-local third-party edit is preserved; an induced copy failure returns a fail-closed error. Integration-test the wiring through `EnsureDefaultWorktreeForChange` (create provisions; reuse backfills while preserving a local edit; an induced provisioning failure surfaces as a fail-closed error from the binding call), and update existing callers for the new signature.
  - depends_on: [t-01, t-02]
  - target_files: [internal/toolgen/worktree_provision_test.go, internal/state/worktree_provision_test.go, internal/state/worktree_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
