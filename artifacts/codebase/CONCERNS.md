# Concerns

- Architectural pressure points:
  - Verification and preflight stages are gate-bearing; context reduction must
    not change the evidence required to pass those gates.
  - Host wording must be runtime-portable: Codex currently does not receive
    generated agent directories, so the template must allow "subagent if
    supported; structured-summary fallback otherwise" rather than depending on a
    single runtime's Task API.
- Brittle areas:
  - `goal-verification` must still record
    `high_risk_check:<domain>.safety_baseline=pass` from a real SAST run when a
    guardrail domain is set.
  - `worktree-preflight` must still record worktree path, branch, and exact
    baseline command references even if the baseline's full output is delegated.
  - `wave-orchestration` must still record task evidence with
    `slipway evidence task`; slimming context cannot permit skipped task
    evidence.
- Migration traps:
  - Editing generated `.codex/`, `.claude/`, `.cursor/`, or `.gemini/` files by
    hand would drift from the template source of truth.
  - Implementing token telemetry or model-profile routing here would broaden the
    issue #114 scope beyond the first context-span optimization.
- Recheck routing:
  - Run focused template tests after editing skill templates, then full
    `go test ./...`.
  - Refresh generated surfaces through the repo-native init/refresh command if
    template tests or repository policy require generated copies to be updated.
- Notes:
  - Source references: `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`,
    `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`,
    `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`.
