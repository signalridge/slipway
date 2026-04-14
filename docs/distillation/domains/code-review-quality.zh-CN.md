# Domain：code-review-quality

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `independent-review` | T1 | hosts `spec-compliance-review`、`code-quality-review`；command `review` |
| `multi-reviewer-calibration` | T1 | host `code-quality-review`；command `review` |

作用：

1. 在 host 层执行 fresh-context、带 verdict contract 的 review。
2. 对多 reviewer 的结果做去重与严重度校准。

说明：

- `independent-review` 是 B1 “host + command 绑定”端到端验证样本。
- `multi-reviewer-calibration` 吸收 `adversarial-reviewer`、`multi-reviewer-patterns` 与 `code-review-ai-ai-review`。
