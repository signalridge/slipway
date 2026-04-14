# Domain：intake-and-framing

在执行开始前，约束意图、范围、上下文与计划包形状的 catalog 技能集合。

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `scope-clarification` | T1 | host `intake-clarification`；`technique-hint` |
| `context-assembly` | T1 | hosts `research-orchestration`、`plan-audit`；`technique-hint` |
| `plan-authoring` | T1 | host `plan-audit`；`host-embedded`；`export-only` |

作用：

1. 作为 S0 / 早期 S1 内核状态的方法层。
2. 在执行前收敛范围、补齐上下文、约束计划包质量。
3. 向外部 adapter 暴露可复用的 authoring 指南（`plan-authoring`）。

说明：

- `scope-clarification` 只是在既有 `intake-clarification` host 之上附着 intake posture，不替代 host。
- `context-assembly` 从 B2 起承载 `hydrate_references[]` 契约；resolver 发出仍待启用。
- `plan-authoring` 吸收了 `superpowers/writing-plans`、`wshobson/workflow-patterns` 与 spec-kitty mission-system 姿态。
