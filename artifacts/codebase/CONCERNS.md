# Concerns

Re-authored for change `resolve-github-issue-137-add-go-source-sast-ci-and-triage-th`
(issue #137). This map scopes to SAST CI and gosec baseline triage.

- Architectural pressure points:
  - A gosec job that exits non-zero on the current full repository baseline would
    make the Security workflow fail immediately on historical findings. User
    clarified all findings must be resolved, so the implementation must fix or
    locally suppress the full baseline before enabling failure.
  - The issue's concrete baseline began as the changed-package scan over `cmd/`,
    `internal/state`, `internal/model`, `internal/toolgen`, and `internal/tmpl`.
    Full `./...` finds additional packages, and those are now in scope because
    the user clarified that all current findings must be resolved.
  - CodeQL for Go is complementary to gosec. It should be added with the
    official CodeQL action path, while gosec remains the baseline-rule authority
    for the issue's named findings.
- Brittle areas:
  - `filepath.WalkDir` callbacks that read or write `path` values are flagged by
    `G122`; blindly changing them can break archive semantics. Fix only when the
    scoped root and symlink behavior remain clear, otherwise suppress with a
    local rationale.
  - `G304` path reads are common in Slipway because the CLI intentionally reads
    artifacts from the current repository/worktree; suppressions must explain
    the authority boundary instead of hiding the rule globally.
  - `G204` git subprocess calls are expected for a git-governance CLI. They
    should use literal executable names and bounded argument construction, with
    local suppressions where gosec cannot infer the boundary.
  - Permission changes must distinguish user-facing tracked artifacts from
    git-scoped runtime state. Private modes are appropriate for local runtime
    evidence/config; repository artifacts may intentionally remain readable.
- Migration traps:
  - Do not edit generated skill copies under `.codex/` or `.claude/` for this
    issue.
  - Do not use global `-exclude=G304,G204,...` rules without rationale; that
    would add a SAST job while preserving an untriaged baseline.
  - Do not make `slipway done` archive/finalize this change; the requested end
    state is `done-ready`.
- Fail-closed requirement:
  - Because this change supports the `irreversible_operations.safety_baseline`
    path, goal verification must include real SAST command evidence and a
    domain/security review. Missing, stale, or inconclusive SAST evidence is a
    blocker.
- Recheck routing:
  - Re-run full-repository gosec after triage and require no unsuppressed
    findings.
  - Re-run full gosec SARIF generation to prove CI upload artifacts can be
    produced by the same policy.
  - Run `go test -count=1 ./...`, then governance `validate` and
    `health --governance` before final closeout.
