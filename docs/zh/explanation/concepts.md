# 核心概念

Slipway 将用户意图、宿主执行和持久 Run 状态分开。理解以下术语即可使用产品，无需先阅读机器协议。

![Slipway Run 生命周期：主动启动后进入每次一个 Action 的 Action/Outcome 循环；用户可以 skip、pause、stop 或 resume；ended 只表示自动 Action 队列为空。](../../assets/diagrams/lifecycle.svg)

## Run

**Run** 是在一个 Git worktree 中实现一个目标的一次可中断尝试。它有范围明确 Action budget、恢复历史和固定的初始 workspace identity。一个任务可以有多个 Run；一个 Run 最多有一个 primary source。

Run 必须主动启动。Slipway 不监听聊天，不把普通对话自动转换成工作，也不通过 ambient hook 启动。

## Action 与 Outcome

CLI 每次返回一个 **Action**：

| Action | 宿主职责 |
| --- | --- |
| `orient` | 检查仓库事实、Git 状态和约定，选择下一步。 |
| `clarify` | 只有仓库事实无法解决时，才询问一个真正的人类决策。 |
| `implement` | 执行范围明确技术修改，并报告实际活动和文件。 |
| `review` | 只读检查观察到的变化，报告意图或质量问题，不修改代码。 |
| `summarize` | 汇总观察到的变化、活动、findings、known issues 和不确定性。 |

Review 默认启用，但只有 Slipway 观察到代码变化后才会签发；`--no-review` 会对该 Run 禁用 Review。

宿主使用结构化 **Outcome** 回答。Slipway 验证并记录 Outcome，独立观察 Git，再选择下一个 Action。宿主报告某项活动不等于活动成功；观察到 diff 也不能证明是谁创建了它。

## 来源

Run 有两种来源：

- **Ad hoc：** 用户给出的 goal 即来源。
- **GitHub Change：** 宿主导入自包含 Change Issue，CLI 按 digest 固定已接受章节。

Run 中的 source snapshot 不会静默变化。刷新 Change Issue 后若已接受内容不同，Slipway 会暂停，要求明确选择保留 pinned snapshot 或采用 candidate。若 amendment 基于另一条 history，则必须启动新 Run。

## Objective 与 Change

**Objective** 是可选的规划结构，用于需要多个独立交付 Change 的结果；它不可执行。

**Change** 是一个有单一连贯结果、可独立实现和交付的工作项。它必须自包含；Run 执行时不会依靠读取父 Objective 来补齐需求。

这些概念与 GitHub 仓库属于个人账号还是 Organization 无关。详见 [GitHub Issue 工作流](../guides/github-issues.md)。

## Budget 与暂停

Action budget 只限制 CLI 可签发多少个 Action，不是时间估算或质量分数。Run 还会因人类决策、来源选择、环境不可用或精确破坏性确认而暂停。

用户可以 skip Action、stop 或 resume Run、调整顺序或接管。Skip 无需理由。Resume 会重新验证初始 worktree identity，并按协议使过期的 pending work 失效。

## Review 与结束

Review 是只读建议。Finding 进入 summary；Slipway 不会自动修复后再次 Review。

`ended` 只表示自动 Action 队列为空，不表示测试通过、Review 无 finding、branch protection 已满足、PR 已批准或 release 已就绪。Slipway 报告事实，由用户和仓库策略决定下一步。

## 本地状态

恢复数据保存在 `<git-common-dir>/slipway/runs/` 下。追加式 journal 记录状态转换；可替换 projection 加速读取；已接受 Issue 章节按内容寻址保存。详见 [Run、恢复与隐私](../guides/runs-and-recovery.md)。

精确 JSON 形状见[机器协议](../reference/machine-protocol.md)。
