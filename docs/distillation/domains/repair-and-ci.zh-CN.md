# Domain：repair-and-ci

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `ci-triage` | T2 | commands `repair`、`status` |
| `review-comment-triage` | T2 | command `repair` |
| `git-recovery` | T2 | commands `repair`、`status`；`worktree-preflight` 失败支持 |

作用：

1. 把 CI 失败与 review comment 转成有界 remediation 计划。
2. 处理 rebase / bisect / reflog / worktree / hook-bypass 等恢复场景。

说明：

- `git-recovery` 吸收了 `git-advanced-workflows`、`spec-kitty-git-workflow` 与 `block-no-verify-hook` 策略。
- 这些技能都不改变 progression authority，只挂在 routed command path 之后。
