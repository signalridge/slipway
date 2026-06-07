# Conventions

- Naming:
  - Governed host skills use `slipway-<skill-name>` in generated surfaces and
    keep the engine skill ID as the lifecycle authority.
  - References to parseable evidence tokens must stay literal, especially
    `high_risk_check:<domain>.safety_baseline=pass` and `baseline_verify_cmd:`.
- File organization:
  - Keep host SKILL.md bodies concise and move long examples or dispatch details
    into `references/` files where the existing skill already supports that
    pattern.
  - Prefer editing `internal/tmpl/templates/skills/...` over generated surfaces.
- Error handling:
  - Gate-bearing instructions should fail closed when delegated evidence is
    missing, stale, inconclusive, or reports blockers.
- Configuration:
  - Do not introduce runtime-specific model or subagent assumptions unless the
    template names a fallback for tools without subagent support.
- State management:
  - Runtime-owned evidence files and task evidence must be written through
    supported commands, not by hand.
- Notes:
  - Current change favors template contract wording and tests over engine schema
    changes.
