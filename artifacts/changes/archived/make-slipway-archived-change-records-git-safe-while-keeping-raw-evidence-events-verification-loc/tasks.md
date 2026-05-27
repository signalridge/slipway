# Tasks

## Task List

- [x] `t-01` Add idempotent Slipway local-state `.gitignore` management.
  - wave: 1
  - depends_on: []
  - target_files: [.gitignore, internal/state/local_ignore.go, internal/state/local_ignore_test.go, internal/bootstrap/init.go, internal/bootstrap/init_test.go, cmd/new.go, cmd/new_test.go, cmd/codebase_map.go, cmd/codebase_map_command_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
  - evidence: verdict
  - acceptance: targeted tests prove init, new, and codebase-map write the managed ignore block idempotently.

- [x] `t-02` Sanitize archived `change.yaml` snapshots while preserving archive discovery.
  - wave: 1
  - depends_on: []
  - target_files: [internal/state/lifecycle.go, internal/state/lifecycle_test.go, internal/state/repair.go, internal/state/store.go, internal/state/store_test.go]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: verdict
  - acceptance: archive tests prove worktree_path is omitted and artifact paths are relative in new archived change.yaml records.

- [x] `t-03` Tolerate intentionally local-only archived lifecycle logs in learning diagnostics.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/learn.go, cmd/learn_test.go]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: verdict
  - acceptance: learn tests prove archived changes without events are analyzed without missing-log signals.

- [x] `t-04` Update operator documentation for the Git boundary.
  - wave: 1
  - depends_on: []
  - target_files: [README.md, docs/operator-guide.md, docs/index.md]
  - task_kind: doc
  - covers: [REQ-005]
  - evidence: checklist
  - acceptance: docs identify Git-managed top-level records and local-only proof directories.

- [x] `t-05` Verify targeted archive, ignore, and learn behavior.
  - wave: 2
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: [internal/state/local_ignore_test.go, internal/state/lifecycle_test.go, internal/state/repair.go, internal/state/store.go, internal/state/store_test.go, internal/bootstrap/init_test.go, cmd/codebase_map_command_test.go, cmd/new_test.go, cmd/learn_test.go]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: verdict
  - acceptance: go test ./internal/state ./internal/bootstrap ./cmd passes.

- [x] `t-07` Close independent-review archive/worktree gaps.
  - wave: 2
  - depends_on: [t-05]
  - target_files: [cmd/done.go, cmd/lifecycle_commands_test.go, cmd/new.go, cmd/new_test.go, internal/state/lifecycle.go, internal/state/lifecycle_test.go, internal/state/paths.go, internal/state/paths_test.go, internal/state/repair.go, internal/state/repair_test.go, internal/state/stats.go, internal/state/stats_test.go, internal/state/store.go, internal/state/store_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - evidence: verdict
  - acceptance: worktree-bound `done --json` reports the owning-worktree archive path when run from the worktree; archive repair sanitizes worktree-local archives in place; archived worktree slugs remain reserved; `new` ensures ignore rules in the default worktree.

- [x] `t-06` Run full repository verification and final Git-safety checks.
  - wave: 3
  - depends_on: [t-07]
  - target_files: [artifacts/changes/archived/make-slipway-archived-change-records-git-safe-while-keeping-raw-evidence-events-verification-loc/**]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: verdict
  - acceptance: go test ./... passes and git checks show raw proof directories are ignored while top-level records are trackable.
