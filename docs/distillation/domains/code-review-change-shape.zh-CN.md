# Domain：code-review-change-shape

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `differential-review` | T1 | command `review` |
| `variant-analysis` | T1 | commands `review`、`repair` |
| `spec-trace` | T1 | host `spec-compliance-review`；commands `validate`、`review` |

作用：

1. 以风险优先级和 blast-radius 感知进行 diff 审查。
2. 搜索已知 bug / vuln 模式的变体。
3. 保持双向 spec-to-code trace。

说明：

- `spec-trace` 是 `spec-compliance-review` 的 host-embedded 技能。
- `variant-analysis` 自然延伸到 `repair`，用于模式驱动修复。
