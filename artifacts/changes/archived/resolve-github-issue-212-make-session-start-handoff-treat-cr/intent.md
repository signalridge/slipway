# Intent

## Summary
Resolve GitHub issue #212: make SessionStart handoff treat a cross-worktree active governed change as informational context instead of an error-shaped hook diagnostic.

## Complexity Assessment
complex
Rationale: the user-visible defect is narrow, but the fix touches generated hook behavior, cross-worktree lifecycle framing, and tests that must preserve explicit `next` / `run` fail-closed semantics.

## Guardrail Domains
None detected.

## In Scope
- Update the generated SessionStart hook surface, primarily `internal/tmpl/templates/hooks/session-start.sh.tmpl`, so `change_bound_to_other_worktree` from read-only handoff is rendered as informational context.
- Ensure the informational handoff names the other active change, its bound worktree, and how to act from the right place or with `--change`.
- Keep generated behavior identical for claude, cursor, gemini, and opencode by fixing the shared hook template rather than host-specific shell copies.
- Add focused tests for the cross-worktree informational path and for preserving diagnostics on genuine failures.
- Preserve explicit `slipway next` and `slipway run` command behavior: wrong-worktree invocations still fail closed with `change_bound_to_other_worktree` / exit 3.

## Out of Scope
- Redesigning the global active-change model or multi-active lifecycle semantics.
- Making explicit lifecycle mutation/query commands silently succeed from the wrong worktree.
- Adding one-off per-host hook workarounds outside the shared generated template surface.
- Changing unrelated handoff, context-pressure, or governance evidence behavior.

## Constraints
- SessionStart remains read-only and must not advance or mutate governed state.
- Real failures such as `slipway root` failure or broken `next --json` output must still render `hook_diagnostic` lines.
- The fix should follow the repo's self-optimization principle by improving the owned generated surface.
- Tests should cover the shell hook behavior without depending on a fragile live workspace layout.

## Acceptance Signals
- A rendered SessionStart hook receiving `change_bound_to_other_worktree` emits an informational line that includes the active change slug, bound worktree, and action hint, with no `hook_diagnostic: slipway next --json failed:` line.
- A rendered SessionStart hook still emits `hook_diagnostic` for genuine `slipway root` or unrelated `slipway next --json` failures.
- Explicit `slipway next --json` and `slipway run --json` from the wrong worktree continue to fail closed with `change_bound_to_other_worktree` and exit 3.
- Template/tool generation tests demonstrate the shared behavior across generated hook hosts.

## Open Questions
None.

## Deferred Ideas
- A future structured handoff-only CLI API could make hooks less dependent on parsing `next --json` error envelopes, but this change should not expand into that redesign unless implementation research shows it is necessary.

## Approved Summary
Confirmed by user on 2026-06-14T11:53:16Z.

Resolve GitHub issue #212 by changing the shared generated SessionStart hook
handoff so `change_bound_to_other_worktree` from the read-only handoff path is
rendered as informational context rather than `hook_diagnostic: slipway next
--json failed`. The informational line must name the other active change, its
bound worktree, and how to act from the right place or with `--change`.

Scope is limited to the shared generated hook surface and focused tests for the
cross-worktree informational path, genuine failure diagnostics, generated host
parity, and explicit command fail-closed behavior. This change will not redesign
the global active-change model, make explicit `next` / `run` succeed from the
wrong worktree, or add per-host shell workarounds.
