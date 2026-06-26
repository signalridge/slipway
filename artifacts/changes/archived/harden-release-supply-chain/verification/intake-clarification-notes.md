# Intake Clarification Notes

## Change

- Slug: `harden-release-supply-chain`
- Source scope: `opt.md` section 2, "release and supply-chain security must close"
- Workflow posture: strict, full quality mode

## User Authorization

The active thread goal directs the agent to:

- choose suitable `opt.md` scopes,
- drive each scope through the governed lifecycle,
- open a PR for each completed change,
- wait for all PR checks to pass before merging,
- pull local `main` after each merge,
- continue to the next change,
- use auto mode for choices or blockers and make the best decision.

That instruction is used as the fresh intake confirmation for this scope.

## Approved Scope

In scope:

- live evidence for `main` protection and release tag protection,
- release workflow tag input validation before secret exposure,
- workflow/job permission minimization,
- protected environment usage for AUR or extra publish jobs where applicable,
- removal of floating workflow actions and floating tool installs named by
  `opt.md`,
- fail-closed GitHub API override validation for REST and GraphQL clients,
- prevention of ambient public GitHub token reuse against override hosts,
- governed `BaseRef` validation before `git worktree add`,
- release-config validation and minimum smoke coverage.

Out of scope:

- changing actual secret values,
- bypassing GitHub branch/tag/ruleset/environment protections,
- `opt.md` section 3 architecture and coverage-gate work except directly needed
  local tests,
- `opt.md` section 4 performance work,
- reopening the lifecycle UX repair already merged in PR #348.

## Acceptance Signals

- live GitHub settings evidence is captured,
- invalid release tags fail before release secrets become visible,
- release/config changes run `goreleaser check` or equivalent validation plus
  snapshot dry-run coverage where applicable,
- cited floating dependencies are replaced by pinned or repo-managed versions,
- targeted tests cover GitHub API override token safety,
- targeted tests cover invalid and option-like `BaseRef`,
- local tests, lint, governed reviews, ship verification, PR CI, merge, and
  local `main` pull complete before moving to the next `opt.md` scope.

## Research Questions

The following questions remain intentionally open and route the change through
research before planning:

- What GitHub branch/ruleset/tag protections are active right now, and which
  gaps can be closed from repository files versus external admin settings?
- Which current release workflows, GoReleaser config, publish channels, and
  smoke scripts exist, and which must be changed for minimum closure?
- Where are GitHub REST/GraphQL clients and `BaseRef` worktree creation wired
  today, and what is the smallest shared validation boundary?
