# Subagent 配置

`subagents` 是 `.slipway.yaml` 里的仓库策略。它告诉 Slipway：每个治理
slot 要向宿主暴露哪个委派目标。Slipway 仍然负责生命周期状态、就绪度、证据和阻塞项；
配置只影响当前 AI 宿主如何运行被委派的会话。

默认 provider 是 `native`。只有当宿主里确实有能执行对应名字的 hub 或适配器时，
才配置 `mcp` 或 `skills`。

## Schema

```yaml
subagents:
  default:
    type: native
    name: default-agent
    session_instructions: Use the host's default fresh session behavior.
    timeout: 30m

  plan_audit:
    name: plan-auditor
    session_instructions: Audit only planning artifacts. Do not edit files.

  executor:
    type: mcp
    name: sliphub-executor
    session_instructions: Execute planned wave tasks and record task evidence.

  review:
    type: skills
    name: sliphub
    session_instructions: Run the selected read-only reviewers in parallel and return separate findings.
    timeout: 45m

  fix:
    name: review-repairer
    session_instructions: Collect all selected reviewer findings before editing files.

  verify:
    name: ship-verifier
    session_instructions: Verify terminal readiness without modifying files.
```

每个 slot 都接受同一组字段：

| 字段 | 含义 |
| --- | --- |
| `type` | Provider family：`native`、`mcp` 或 `skills`。留空等价于 `native`。 |
| `name` | provider 自己拥有的目标名字。对 `native` 来说，它是宿主支持的 agent 名；对 `mcp` / `skills` 来说，它是该 provider 选择的 hub、tool 或 skill 入口。 |
| `session_instructions` | 给被委派会话的自然语言指令。它不是 provider profile，也不是模型 prompt 继承。 |
| `timeout` | 可选的宿主侧超时提示。Slipway 只校验前后空白；具体解释由宿主/provider 决定。 |

`mcp` 和 `skills` 的有效配置必须有非空 `name`。如果某个 slot 把 `type`
改成了不同于 `default` 的 provider family，也要在这个 slot 上设置 `name`；
名字不会跨 provider family 继承。

## Slots

| Slot | JSON 输出面 | 说明 |
| --- | --- | --- |
| `default` | 被其他 slot 继承 | 共享兜底。如果完全没有 `subagents` 配置，Slipway 不输出委派 directive。 |
| `plan_audit` | `plan-audit` 的 `next_skill.subagent` | plan 编写本身仍在主会话里；只有 plan audit 被委派。 |
| `executor` | `input_context.wave_plan.executor_subagent` | S2 wave 执行。provider 可以在内部 fan out，但 Slipway 仍审计任务证据和 changed files。 |
| `review` | 已选 S3 reviewers 的 `next_skill.subagent` 和 `review_batch.subagent` | 一个 slot 覆盖整个已选 review batch。不配置逐 reviewer 的 provider family。 |
| `fix` | `slipway fix --json` 的 `contract.subagent` | S3 review findings 的 fresh repair session。 |
| `verify` | `ship-verification` 的 `next_skill.subagent` | 终端只读验证。 |

这里刻意没有 `plan` slot，也没有 substep 级别配置。Planning 是高上下文的编写工作，
保留在主会话里。Subagent 配置只从 Slipway 有明确独立性或派发边界的地方开始。

## 不配置什么

Provider 私有工具权限、模型参数，以及任意 provider 参数，都不是 Slipway 的用户配置面。
当前 slot 需要什么工具边界，由 Slipway 和被选中的 provider 自己决定。如果某个 hub
需要路由细节，把操作意图写进 `session_instructions`，让 provider 解释。

这样 `.slipway.yaml` 保持稳定，同时 `mcp` 和 `skills` provider 仍然可以在自己的
命名目标背后支持差异很大的内部选项。

## config 命令示例

把 slot 切到 `mcp` 或 `skills` 之前，先设置 `name`，因为每次 `set` 之后都会校验
整个配置文件：

```bash
slipway config set subagents.review.name sliphub
slipway config set subagents.review.type skills
slipway config set subagents.review.session_instructions "Run selected reviewers in parallel and return separate findings."
slipway config set subagents.review.timeout 45m
```

要一次配置多个 slot，直接写 YAML 仍然最清楚。

## 重新生成宿主面

修改 subagent 配置后，运行 `slipway init --refresh`，让生成的适配器面和 hook
与当前 CLI contract 对齐。
