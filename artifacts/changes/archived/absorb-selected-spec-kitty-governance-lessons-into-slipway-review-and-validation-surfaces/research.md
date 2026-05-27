# Research

## Research Findings

### Architecture
- Affected modules: `internal/engine/wave`, `internal/model/execution_summary.go`, `internal/engine/governance`, `cmd/validate.go`, `cmd/review.go`, `cmd/status_view_build.go`, and host-skill templates under `internal/tmpl/templates/skills`.
- Dependency chains: task plan parsing -> execution evidence summary -> governance readiness/status/review surfaces.
- Blast radius: bounded to governance evaluation and host guidance; no lifecycle state authority rewrite.
- Constraints: preserve `change.yaml` as current-state authority and lifecycle events as append-only trace.

### Patterns
- Existing conventions: task metadata is parsed from checkbox-native `tasks.md`; execution task evidence already carries `changed_files` and `target_files`.
- Reusable abstractions: `wave.ParseTaskPlan`, `model.ExecutionSummary`, `model.TaskRun`, reason codes, and existing validation/status blocker reporting.
- Convention deviations: a new evaluator package is acceptable because scope drift is distinct from blast-radius signal derivation.

### Risks
- Technical risks: medium false-positive risk if generated/artifact files are not declared in `target_files`; medium false-negative risk if changed-files evidence is omitted.
- Guardrail domains: none.
- Reversibility: remove the evaluator and call sites without changing persisted change authority or execution summary schema.

### Test Strategy
- Existing coverage: task plan parsing, execution summary validation, validate/status/review readiness tests, template capability tests.
- Infrastructure needs: focused evaluator tests and CLI/readiness fixture tests.
- Verification approach: targeted package tests first, then `go test ./...` before closeout if runtime permits.

### Slipway Current State
- `tasks.md target_files` is already mandatory after plan audit and is parsed by the checkbox-native task plan parser.
- `internal/engine/control/derive.go` uses planned target files before execution and changed files after execution only to derive blast radius.
- `internal/engine/governance/health.go` loads planned target files from `tasks.md` and passes them to control derivation.
- `execution_summary.tasks[].changed_files` is already carried through task evidence parsing and summary models.
- There is no current planned-vs-actual reconciliation gate that reports changed files outside planned target files.
- `codebase-map` output currently uses repo-root artifact paths even when invoked from a dedicated worktree, which can mutate the main checkout's `artifacts/codebase` from worktree execution.

### spec-kitty Source Pattern
- `spec-kitty` treats `owned_files` as a first-class ownership declaration in work-package metadata.
- That declaration feeds lane conflict calculation, worktree topology, prompt/context selection, and commit-guard enforcement.
- The useful primitive for Slipway is not lanes; it is the single declared file boundary reused by downstream checks.

### Slipway Adaptation
- Slipway should reuse existing `tasks.md target_files` rather than introducing WP frontmatter or a new ownership DSL.
- The first implementation should be read-only reconciliation and fail-closed reporting after execution evidence exists.
- Operation-level allowances, hook enforcement, and context pruning should wait until file-boundary matching semantics are tested and stable.
- Because codebase maps are updateable artifacts, `artifacts/codebase` should follow the active workspace/worktree in the same way change artifacts do when a worktree is bound.

## Risks
- False positives if generated files, governed artifact files, or verification outputs are not represented in `target_files`.
- False negatives if execution evidence omits `changed_files`.
- Overreach if scope checking is mixed into blast-radius derivation instead of staying a distinct governance contract.
- Host-skill wording drift if generated templates are updated without tests.

## Alternatives Considered

- Copy spec-kitty `owned_files` frontmatter and `OwnershipManifest` directly.
  - Decision: rejected.
  - Reason: Slipway already has task-level `target_files`; adding WP frontmatter would duplicate authority and pull in spec-kitty's product shape.

- Implement lane scheduling and worktree-per-lane conflict handling.
  - Decision: rejected.
  - Reason: this solves multi-lane orchestration, not the current scope-containment gap.

- Add a Claude Code PreToolUse hook first.
  - Decision: deferred.
  - Reason: shift-left enforcement is useful but should be based on a proven read-only reconciliation engine to avoid noisy blocks.

- Add contract round-trip review first.
  - Decision: partially deferred.
  - Reason: review contract evidence is useful, but file-boundary reconciliation is the stronger current gap. Template wording can be tightened in this change without making review prose the core gate.

- Add Scope Contract reconciliation as a distinct evaluator.
  - Decision: selected.
  - Reason: it uses existing Slipway artifacts, preserves current state authority, and directly detects AI scope drift.

- Keep `artifacts/codebase` rooted at the main checkout.
  - Decision: rejected.
  - Reason: codebase maps are updateable artifacts; worktree execution should not mutate the main checkout behind the user's back.

## Decision
Implement Scope Contract reconciliation as a Slipway-native evaluator over `tasks.md target_files` and execution changed-file evidence. Surface drift as governance blocker/reporting in validation/review readiness. Make updateable `artifacts/codebase` output prefer the active workspace/worktree. Keep hook enforcement, context-pruning, glossary, schema migration, and operation-level permissions as deferred follow-ups.

## Unknowns
- Resolved: Should Slipway copy `spec-kitty` lane/work-package ownership directly? -> No; reuse the file-boundary primitive while keeping Slipway's existing task plan authority.
- Resolved: Should hook enforcement ship first? -> No; start with deterministic read-only reconciliation and add hook enforcement later.
- Remaining: None.

## Assumptions
- Existing task plan `target_files` is the right first contract source. Evidence: `internal/engine/wave/parse.go` and current plan validation already require it after plan audit.
- Existing execution evidence changed-files is the right actual source. Evidence: `internal/model/execution_summary.go` and `internal/engine/progression/wave_sync.go`.
- Platform/lane features are intentionally out of scope. Evidence: `intent.md#Out of Scope` and `decision.md#Alternatives Considered`.

## References
- `internal/engine/wave/parse.go`: task plan parser and accepted metadata keys.
- `internal/engine/control/derive.go`: current planned/actual file counts are blast-radius-only.
- `internal/model/execution_summary.go`: execution task changed-files data model.
- `cmd/codebase_map.go`: codebase-map command output path selection.
- `cmd/common.go`: invocation workspace/root helpers for routed commands.
- `internal/state/paths.go`: artifact path resolution for bound worktrees.
- `spec-kitty/src/specify_cli/ownership/models.py`: source ownership declaration pattern.
- `spec-kitty/src/specify_cli/policy/commit_guard.py`: source file-boundary enforcement pattern.

## Canonical References
- `internal/engine/wave/parse.go`
- `internal/engine/control/derive.go`
- `internal/engine/governance/health.go`
- `internal/model/execution_summary.go`
- `internal/engine/progression/wave_sync.go`
- `cmd/codebase_map.go`
- `cmd/common.go`
- `internal/state/paths.go`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `spec-kitty/src/specify_cli/ownership/models.py`
- `spec-kitty/src/specify_cli/policy/commit_guard.py`
