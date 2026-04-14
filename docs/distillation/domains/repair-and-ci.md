# Domain: repair-and-ci

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `ci-triage` | T2 | commands `repair`, `status` |
| `review-comment-triage` | T2 | command `repair` |
| `git-recovery` | T2 | commands `repair`, `status`; failure support for `worktree-preflight` |

Role:

1. Turn CI failures and review comments into bounded remediation plans.
2. Recover from rebase / bisect / reflog / worktree / hook-bypass problems.

Notes:

- `git-recovery` absorbs `git-advanced-workflows`, `spec-kitty-git-workflow`,
  and `block-no-verify-hook` policy.
- None of these skills alter progression; they sit behind routed command paths.
