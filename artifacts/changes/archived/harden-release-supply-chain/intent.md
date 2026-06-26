# Intent

## Summary
harden release supply chain

## Complexity Assessment
complex

This change spans GitHub repository protection settings, release workflow
privileges, third-party action/tool pinning, API token routing, git ref
validation, and release smoke coverage. It is security-sensitive and requires
fresh live evidence plus full governed review.

## Guardrail Domains
- Release/security workflow hardening
- External API token handling
- Irreversible repository protection configuration

## In Scope
- Verify current live GitHub protection for `main` and release tags, and add or
  document the repo-owned configuration/evidence needed to close gaps.
- Harden release workflow tag validation so untrusted manual inputs fail before
  release secrets are visible, and validated tag output becomes the only release
  ref/version input.
- Minimize workflow/job permissions and put AUR or extra publishing behind a
  protected environment where the repo can enforce approval.
- Replace floating workflow action and tool dependencies called out by
  `opt.md` section 2.3 with pinned actions or repo-managed versions plus an
  update path.
- Make `SLIPWAY_GITHUB_API_URL` and shared REST/GraphQL GitHub client creation
  fail closed for unknown or unsafe override hosts, and prevent ambient
  `GH_TOKEN`/`GITHUB_TOKEN` from being sent to override hosts by default.
- Validate governed `BaseRef` values before they reach `git worktree add`,
  rejecting option-like or invalid refs with a product-owned remediation.
- Add release-config PR checks and release smoke coverage sufficient to catch a
  broken artifact graph before tag-only release time.

## Out of Scope
- Do not modify actual secret values.
- Do not bypass GitHub ruleset, branch protection, tag protection, protected
  environment, or manual approval controls.
- Do not mix in `opt.md` section 3 architecture/coverage-gate work unless a
  small local test hook is directly required by this hardening.
- Do not mix in `opt.md` section 4 status-read performance work.
- Do not reopen the already merged lifecycle UX repair from PR #348.

## Constraints
- Use the bound worktree
  `/Users/yixianlu/ghq/github.com/signalridge/slipway/.worktrees/harden-release-supply-chain`.
- Preserve unrelated root dirt in the main checkout.
- Treat GitHub live state as drift-prone and verify it during the change.
- If a protection rule requires admin-only web configuration that cannot be
  applied locally, record the live gap and provide repo-owned evidence or
  automation where possible instead of pretending it is complete.
- Run the normal Go tests, lint, governed review, ship verification, PR checks,
  and merge/pull loop before moving to the next `opt.md` scope.

## Acceptance Signals
- Live `gh api` evidence shows `main` is protected by branch protection or an
  active ruleset, and release tags have a restrictive creation/update rule or a
  tracked remediation artifact captures the required admin change.
- Invalid manual release tags fail before any release secret is available.
- Release/config changes run `goreleaser check` or an equivalent validation and
  a snapshot dry run where applicable.
- The workflows named in `opt.md` section 2.3 no longer use the cited floating
  dependencies, and tool versions are pinned or repo-managed.
- Tests prove unsafe GitHub API override URLs fail closed and ambient public
  GitHub tokens are not sent to override hosts.
- Tests prove invalid or option-like `BaseRef` values do not reach
  `git worktree add`, while valid branches, tags, and SHAs remain usable.
- Full local verification, governed review, ship verification, and PR CI pass
  before merge.

## Open Questions
- [x] What GitHub branch/ruleset/tag protections are active right now, and which
  gaps can be closed from repository files versus external admin settings?
- [x] Which current release workflows, GoReleaser config, publish channels, and
  smoke scripts exist, and which must be changed for the minimum closure?
- [x] Where are GitHub REST/GraphQL clients and `BaseRef` worktree creation wired
  today, and what is the smallest shared validation boundary?

## Deferred Ideas
- Broader architecture dependency cleanup from `opt.md` section 3.
- Status-read performance and caching work from `opt.md` section 4.
- A future dedicated change for release-channel expansion beyond configured
  channels.

## Approved Summary
User authorization source: the active thread goal instructs the agent to keep
selecting suitable `opt.md` scopes, use governed flow, open a PR for each
change, merge only after all PR checks pass, pull local `main`, and continue to
the next change. It also grants auto-mode permission for choices and blockers so
the agent should make the best decision without stopping for ordinary
clarifications.

This change closes `opt.md` section 2 as one release and supply-chain
hardening scope. It covers live GitHub protection evidence, release workflow
input and permission hardening, third-party action/tool pinning, GitHub API
override token safety, `BaseRef` validation before `git worktree add`, and
release smoke coverage for release-config changes.

Out of scope: actual secret value changes, bypassing GitHub protection or
manual approval controls, `opt.md` section 3 architecture/coverage-gate work
except for directly required local tests, `opt.md` section 4 performance work,
and the already merged lifecycle UX repair from PR #348.

Primary acceptance signal: live settings evidence, targeted regression tests,
workflow validation, local test/lint, governed review, ship verification, and
PR CI all pass before the PR is merged and local `main` is pulled.
