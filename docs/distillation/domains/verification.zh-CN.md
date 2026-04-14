# Domain：verification

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `coverage-analysis` | T1 | command `validate`；host `goal-verification` |
| `property-testing` | T1 | command `validate`；host `goal-verification` |
| `mutation-testing` | T1 | command `validate`；host `goal-verification` |
| `performance-profiling` | T1 | commands `validate`、`status`；host `goal-verification` |

作用：

1. 向 `goal-verification` 提供新鲜且方法匹配的验证证据。
2. 对长周期验证工作通过 `status` 输出诊断式摘要（`performance-profiling`）。

说明：

- 每个 verification 技能都输出可被 `goal-verification` 消费的结构化 verdict。
- `mutation-testing` 带有 `tool-recipe` 附着（典型驱动：`pit`、`mutmut`、`stryker`、`gomutation`）。
