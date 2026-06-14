# Decision

## Alternatives Considered

1. Classify `change_bound_to_other_worktree` inside the shared SessionStart hook template and render an informational handoff line.
   - Pros: smallest blast radius, covers all generated hook hosts through one template, preserves explicit command fail-closed behavior, and matches the confirmed issue scope.
   - Cons: requires narrow shell-side parsing of the structured JSON error envelope.
2. Add a new handoff-only CLI command or flag that returns a non-error local-handoff envelope and call that from the hook.
   - Pros: cleaner long-term API boundary and less shell parsing.
   - Cons: larger CLI surface and tests for a narrowly scoped defect; risks expanding this issue into lifecycle API design.
3. Change `slipway next --json` itself to succeed when the active change is bound elsewhere.
   - Pros: hook becomes simpler.
   - Cons: directly violates the acceptance criterion that explicit `next` / `run` remain fail-closed from the wrong worktree.

## Selected Approach

Use alternative 1. The shared SessionStart hook template will classify only the structured `change_bound_to_other_worktree` error emitted by `slipway next --json` and convert that case into a compact informational line. All other `next --json` failures remain hook diagnostics.

This keeps the lifecycle command contract unchanged while fixing the read-only handoff framing at the generated surface that every hook host shares.

## Interfaces and Data Flow

- Input stays the existing `slipway next --json` command executed by the hook from `hook_cwd`.
- On success, the hook continues to pass through `next_json` unchanged.
- On failure, the hook checks whether the output contains the structured `change_bound_to_other_worktree` envelope and a `bound_changes` entry.
- For that single case, the hook writes an informational field such as `session_handoff_info: no active change in this worktree; active change <slug> is bound to <path>; cd there or use --change <slug> to act`.
- For every other failure, the hook preserves the existing `hook_diagnostic: slipway next --json failed:` behavior.
- No Go command interface changes are required. `resolveActiveChangeRef` remains the command-level authority for fail-closed behavior.

## Rollout and Rollback

Rollout is covered by updating the shared template and tests. Generated host copies are not hand-edited.

Rollback is a normal source revert of the shared template and tests. Verification command for rollback or implementation is:

```bash
go test ./internal/tmpl ./cmd
```

## Risk

- Shell parsing risk: the hook must avoid pretending to be a full JSON parser. Mitigation: classify only the stable `error_code` and simple `bound_changes` fields used in the existing error envelope.
- Hidden-failure risk: the hook must not suppress unrelated errors. Mitigation: tests keep an unrelated `next --json` failure as `hook_diagnostic`.
- Host drift risk: hand-editing generated copies would diverge. Mitigation: edit only `internal/tmpl/templates/hooks/session-start.sh.tmpl` and verify rendered template behavior.
- Command regression risk: explicit `run` / `next` could accidentally be relaxed. Mitigation: command tests pin `change_bound_to_other_worktree` resolution.
