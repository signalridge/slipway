# Domain：ops-and-diagnostics

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `incident-response` | T3 | commands `status`、`health`；`export-only` |

作用：

1. 进行严重度分级、时间线重建与 PIR 流程推进。
2. 不进入 `repair`；T3 在 governed code path 保持只读诊断属性。

说明：

- 吸收 `incident-commander`、`incident-response` 与 `acceptance-orchestrator` 的 gate posture。
- 消费 `observability-query`、`sentry` 等 view-only 面作为证据，不再包一层薄 wrapper。
