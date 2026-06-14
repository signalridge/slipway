# Assurance

## Scope Summary

Delivered GitHub issue #212 by changing the shared generated SessionStart hook
template so a read-only `slipway next --json` result with
`change_bound_to_other_worktree` is rendered as informational handoff context
instead of `hook_diagnostic: slipway next --json failed`.

The informational line names the active change, its bound worktree, and the
action hint to `cd` there or use `--change`. Explicit lifecycle commands remain
fail-closed from the wrong worktree; no generated host copy was hand-edited.

## Verification Verdict

Current execution evidence is fresh at `run_summary_version: 1`.

- `wave-orchestration`: pass
- `spec-compliance-review`: pass
- `code-quality-review`: pass
- Task evidence: `t-01` through `t-04` pass
- Scope contract: pass

Fresh verification commands passed:

- `bash -n internal/tmpl/templates/hooks/session-start.sh.tmpl`
- `go test -count=1 ./internal/tmpl ./internal/toolgen ./cmd`

## Evidence Index

- `verification/execution-summary.yaml`: run summary version 1, all planned tasks pass.
- `verification/wave-orchestration.yaml`: wave execution evidence with `dispatch_mode:wave=1:degraded_sequential`.
- `verification/wave-orchestration-notes.md`: implementation root cause, changed-file conflict check, and verification commands.
- `verification/spec-compliance-review.yaml`: `layer:R0=pass`, `scope_contract:pass`, `negative_path:pass`, `decision_fidelity:pass`.
- `verification/spec-compliance-review-notes.md`: bidirectional requirements-to-code trace.
- `verification/code-quality-review.yaml`: `layer:IR1=pass`, `toolchain_compat:pass`.
- `verification/code-quality-review-notes.md`: implementation quality, test quality, safety, and generated-surface compatibility review.

## Requirement Coverage

- REQ-001: Covered by `internal/tmpl/templates/hooks/session-start.sh.tmpl` and `TestSessionStartHookTreatsBoundWorktreeChangeAsInformational`.
- REQ-002: Covered by unchanged shared active-change resolution plus `TestResolveActiveChangeRefReportsBoundElsewhereFromRoot`, `TestNextChangeFlagFromRootTargetsBoundWorktree`, and `TestRunFromRootReportsBoundElsewhere`.
- REQ-003: Covered by the preserved diagnostic branch plus `TestSessionStartHookSurfacesNextFailureDiagnostic` and `TestSessionStartHookSurfacesRootFailureDiagnostic`.
- REQ-004: Covered by the shared template change, `renderSessionHook`, `TestRenderSessionStartHookTemplate`, adapter contract tests, and `go test ./internal/toolgen`.

## Residual Risks and Exceptions

- The hook uses narrow shell parsing of Slipway's own structured JSON error envelope. This is accepted because it only classifies one stable `error_code`, requires both `slug` and `worktree_path`, and falls back to diagnostics for unrelated or incomplete output.
- No dependency, manifest, lockfile, external API, schema, auth, secret, or network behavior changed.
- Wave 1 was planned as parallel but executed as degraded sequential because subagent spawning requires an explicit user request. This is recorded in wave evidence and introduced no changed-file overlap.

## Rollback Readiness

Rollback is a normal source revert of the shared SessionStart hook template and
the focused tests. No migration, generated copy cleanup, dependency downgrade,
or external coordination is required.

Rollback verification should run:

- `bash -n internal/tmpl/templates/hooks/session-start.sh.tmpl`
- `go test -count=1 ./internal/tmpl ./internal/toolgen ./cmd`

## Archive Decision

Ready to proceed toward done-ready after active `validate --json` and lifecycle
readiness proof are captured in the active worktree. The active bundle has not
been archived yet, and this assurance does not claim archived-bundle
revalidation.
