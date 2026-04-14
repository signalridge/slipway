# Domain：execution-discipline

约束 S2 执行阶段保持可验证性的 catalog 技能集合。

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `tdd-proof` | T1 | hosts `tdd-governance`、`wave-orchestration`；`technique-hint` |
| `parallel-executor-contract` | T1 | host `wave-orchestration` |
| `fresh-verification-evidence` | T1 | hosts `goal-verification`、`final-closeout`、`tdd-governance` |

作用：

1. 保持 guardrail-domain 路径 test-first。
2. 约束并行子代理分派的边界与交接。
3. 没有 fresh commands + fresh evidence 时禁止完成声明。

说明：

- `tdd-proof` + `fresh-verification-evidence` 共同增强 `tdd-governance` host，不替代 host。
- `parallel-executor-contract` 吸收了 `dispatching-parallel-agents`、`subagent-driven-development` 与 spec-kitty implement-review 姿态。
