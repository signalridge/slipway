# Tasks

## Task List

- [x] `t-01` Model grammar: make `fix` a multi-valued context-origin stage. In internal/model/context_attestation.go add an explicit single-vs-multi-valued stage classifier (only `fix` is multi-valued) and update `ContextOriginHandlesFromVerification` so the same-stage fail-closed guard applies to single-valued stages only — multi-valued `fix` references no longer poison the whole-record parse and are not stored in the single-valued map. Add `FixContextOriginHandleSetFromVerification(record)` that returns the deduplicated set of every recorded `stage=fix` handle (set semantics, never fail-closed, non-nil empty set when absent), mirroring `ExecutorParticipantHandleSetFromVerification`. Confirm `ReviewContextOriginHandleFromVerification` now resolves the unique review handle when multiple `stage=fix` handles coexist. In context_attestation_test.go ADD cases (review handle resolves alongside multiple distinct fix handles; record with only multiple fix handles yields no review handle without failing closed; fix set extraction dedup; empty fix set) and KEEP the existing single-valued same-stage fail-closed cases green (do not relax them).
  - depends_on: []
  - target_files: ["internal/model/context_attestation.go", "internal/model/context_attestation_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Authority feeder: collect the complete fix handle set and stop the false reviewer-missing blocker. In internal/engine/progression/authority.go rewrite the `crossStageContextParticipants` fix-stage block to union `model.FixContextOriginHandleSetFromVerification` across every selected reviewer's passing record into the `fix` participant `HandleSet` (instead of reading a single `handles[fix]` via the now-single-valued map). The selected-reviewer loop is unchanged: because the model parse no longer fails closed on multi-fix, `ReviewContextOriginHandleFromVerification` resolves and the `context_origin_handle_invalid` reviewer-missing blocker is no longer emitted for the multi-fix shape. In authority_test.go ADD coverage: a passing reviewer record with one review handle plus multiple distinct fix handles produces no reviewer-missing blocker and a `fix` participant `HandleSet` containing every fix handle.
  - depends_on: [t-01]
  - target_files: ["internal/engine/progression/authority.go", "internal/engine/progression/authority_test.go"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-03` Fix command instructions: in internal/tmpl/templates/_partials/command-fix-body.tmpl make explicit that a reviewer's evidence may accumulate multiple `context_origin:stage=fix=<handle>` references — one per fresh-context repair subagent / batch — without invalidating the single `context_origin:stage=review` handle. In internal/tmpl/templates_test.go add a contract assertion pinning the new multi-fix wording, keeping the existing fix-body reexecution-mode assertions intact.
  - depends_on: []
  - target_files: ["internal/tmpl/templates/_partials/command-fix-body.tmpl", "internal/tmpl/templates_test.go"]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Integration verification: run focused (`go test ./internal/model/... ./internal/engine/progression/... ./internal/tmpl/...`), repository-wide (`go test ./...`), and lint (golangci-lint, incl. gofmt simplify) gates from the current worktree; confirm end-to-end that reviewer evidence with multiple `stage=fix` handles plus one `stage=review` handle no longer yields the "recorded no context-origin handle for selected reviewer" diagnostic; record implementation evidence.
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["artifacts/changes/fix-the-misleading-reviewer-context-origin-diagnostic-report/verification/implementation.md"]
  - task_kind: verification
  - covers: [REQ-004, REQ-006]
