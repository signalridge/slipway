# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/tmpl/templates/hooks/session-start.sh.tmpl` is the shared generated SessionStart hook template. It currently calls `slipway next --json` read-only and treats any non-zero result as `hook_diagnostic: slipway next --json failed` (`internal/tmpl/templates/hooks/session-start.sh.tmpl:37-53`).
- Dependency chain: `toolgen.ToolConfig.SessionHook` maps the same SessionStart hook template into `.claude/hooks/slipway-session-start.sh`, `.cursor/hooks/slipway-session-start.sh`, `.gemini/hooks/slipway-session-start.sh`, and `.opencode/hooks/slipway-session-start.sh` (`internal/toolgen/toolgen.go:45-124`).
- Command error source: wrong-worktree active-change resolution is intentionally produced by `wrapResolutionError`, which emits `error_code=change_bound_to_other_worktree`, `details.bound_changes[]`, and remediation with `--change` / `cd` guidance (`cmd/common.go:546-571`).
- Explicit command callers: `run` resolves the active change before mutation or advancement, so preserving `resolveActiveChangeRef` behavior preserves fail-closed command semantics (`cmd/run.go:35-38`).
- Blast radius: shared hook template rendering and hook behavior tests. The explicit command behavior should be pinned, not redesigned.
- Constraints: SessionStart must remain read-only; genuine root/next failures must still surface as diagnostics; generated host parity comes from the shared template, not checked-in per-host generated copies.

### Patterns
- Existing hook pattern: `run_slipway_readonly` captures command output/status and keeps hook execution non-fatal to the shell (`internal/tmpl/templates/hooks/session-start.sh.tmpl:37-46`).
- Existing diagnostics pattern: root failures are normalized into `hook_diagnostic` while preserving compact output (`internal/tmpl/templates/hooks/session-start.sh.tmpl:12-23`).
- Existing test pattern: `internal/tmpl/hooks_behavior_test.go` renders the template, injects a fake `slipway` executable, and asserts observable shell output and delegated commands (`internal/tmpl/hooks_behavior_test.go:176-254`).
- Existing command-level baseline: `cmd/active_change_resolution_test.go` already verifies `change_bound_to_other_worktree` includes slug, worktree path, and `--change` remediation for root invocations (`cmd/active_change_resolution_test.go:16-39`).
- Codebase map relevance: `artifacts/codebase/ARCHITECTURE.md`, `TESTING.md`, and `CONCERNS.md` are populated for issue #203, not this hook issue, so they are stale for planning authority and only useful as a warning that populated map status does not imply relevance.

### Risks
- Technical risks:
  - Medium: parsing JSON in portable shell can become brittle. Keep classification narrow to `error_code=change_bound_to_other_worktree` and use the existing structured error envelope fields.
  - Medium: suppressing too much could hide real `next --json` failures. Only this one known precondition error should become informational; all other failures remain `hook_diagnostic`.
  - Low: output length could grow. Keep the informational line compact and re-run the existing payload-size assertion.
  - Low: generated host drift. Fixing the shared template should cover all generated hosts; tests should assert the template contains the classifier/rendering path.
- Guardrail domains: none. This is a read-only developer workflow surface.
- Reversibility: high. The change is confined to generated hook template behavior and tests.

### Test Strategy
- Existing coverage:
  - `TestSessionStartHookSurfacesNextFailureDiagnostic` verifies unrelated `next --json` failure remains a diagnostic (`internal/tmpl/hooks_behavior_test.go:176-211`).
  - `TestSessionStartHookSetsToolEnvForReadOnlyCommands` verifies successful `next --json` output is passed through, remains compact, and does not call status/hook-lite/preview variants (`internal/tmpl/hooks_behavior_test.go:213-254`).
  - `TestRenderSessionStartHookTemplate` verifies template rendering and guards against retired command variants (`internal/tmpl/templates_test.go:365-379`).
  - `TestResolveActiveChangeRefReportsBoundElsewhereFromRoot` verifies explicit active-change resolution remains fail-closed for bound worktrees (`cmd/active_change_resolution_test.go:16-39`).
- Infrastructure needs: no live multi-worktree fixture is required for hook behavior; a fake `slipway` executable can return the exact JSON error envelope.
- Verification approach:
  - Add a hook behavior regression where fake `slipway next --json` exits 3 with `change_bound_to_other_worktree`; assert an informational line names slug/worktree/action and no failed diagnostic appears.
  - Keep the unrelated failure diagnostic test unchanged or strengthened.
  - Add or extend command-level coverage so explicit `run` from the wrong worktree still returns `change_bound_to_other_worktree`.
  - Run focused `go test ./internal/tmpl ./cmd` and broaden before closeout.

### Options
- Option A: shared hook template classifies the existing structured `change_bound_to_other_worktree` error and renders informational handoff text.
  - Tradeoffs: smallest change, preserves command semantics, covers all hook hosts through the shared generated template. It relies on narrow shell parsing but avoids new CLI surface area.
- Option B: add a new handoff-only CLI flag/command that returns a non-error `no_local_active_change` envelope, then call that from the hook.
  - Tradeoffs: strongest product boundary and avoids shell-side error classification, but expands CLI API and test surface beyond the issue's immediate defect.
- Option C: alter `slipway next --json` itself to return success when active changes are bound elsewhere.
  - Tradeoffs: simple for the hook, but violates explicit command fail-closed acceptance criteria and is rejected.
- Selected: Option A, aligned with the user's confirmed issue scope. It fixes the owned shared generated surface, preserves explicit `next` / `run` behavior, and keeps the change reversible.

## Unknowns
- Resolved: Is the populated codebase map relevant to this issue? -> No. It is re-authored for issue #203, so this research uses source/test citations as planning authority.
- Resolved: Does generated host parity require editing each checked-in host copy? -> No. `toolgen.ToolConfig` points claude/cursor/gemini/opencode at the same generated SessionStart hook template.
- Remaining: None.

## Assumptions
- The fake-hook test can stand in for a live cross-worktree setup because the issue is about the hook's classification of the structured `next --json` error envelope, not the store's worktree discovery itself. Evidence: existing hook behavior tests already use fake `slipway` scripts for hook-output contracts.
- The existing command-level active-change resolution path is authoritative for explicit `next` and `run` fail-closed semantics. Evidence: `cmd/common.go:546-571`, `cmd/run.go:35-38`, and `cmd/active_change_resolution_test.go:16-39`.

## Canonical References
- `https://github.com/signalridge/slipway/issues/212`
- `internal/tmpl/templates/hooks/session-start.sh.tmpl:37`
- `internal/tmpl/templates/hooks/session-start.sh.tmpl:49`
- `internal/tmpl/templates/hooks/session-start.sh.tmpl:52`
- `internal/tmpl/hooks_behavior_test.go:176`
- `internal/tmpl/hooks_behavior_test.go:213`
- `internal/tmpl/templates_test.go:365`
- `internal/toolgen/toolgen.go:45`
- `cmd/common.go:546`
- `cmd/run.go:35`
- `cmd/active_change_resolution_test.go:16`
