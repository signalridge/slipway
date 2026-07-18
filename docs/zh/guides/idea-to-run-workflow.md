# 从想法到 Run 的工作流

`slipway-workflow` 是一个必须由用户明确调用的宿主侧桥梁：它把粗略想法推进为 Slipway work item 草稿。AI 可以自主完成调查、访谈结构与综合，但这些活动不会被硬化成持久治理流水线；它不增加 CLI 命令、Run 状态、质量 gate 或自动修复循环。

下文使用 capability 名称，不限定各宿主的调用语法；具体入口见[宿主适配器](../reference/adapters.md)。

## 权限边界

一次明确的 `slipway-workflow` 调用只授权第一行：

| 阶段 | AI 可自主完成 | 仍由用户拥有 |
| --- | --- | --- |
| 调查与草拟 | 读取仓库事实、研究范围明确的未知项、组织访谈、选择 Change 或 Objective、综合草稿 | 真正的产品/风险决策；任何另行请求的 artifact 写入 |
| 发布 | 不会自动执行 | 单独调用 `slipway-propose`，并对其精确外部写入计划做一次当前确认 |
| 拆分 | 不会自动执行 | 对已发布 Objective 单独调用 `slipway-decompose` |
| 执行 | 不会自动执行 | 单独调用 `slipway-run`，选择来源并给出初始 budget |

这里的“无状态”只表示 workflow 不创建 Slipway Run、journal 或跨阶段 cursor。对话、文档、tracker Issue、prototype 与代码修改仍然都是状态或副作用。默认的想法到草稿路径是只读的；只有用户原始请求已经授权某个 artifact 及其范围时，才可以绕行创建它，而且必须如实报告，不能把它直接当作可执行来源。

## AI 自主的前半段

宿主先检查当前 Git 状态、相关代码与测试、以及仓库的验证约定。能够自行发现的事实不得拿来询问用户。确有只能由人决定的问题时，每次只问一个，基于已观察事实推荐选项，说明替代方案与取舍，然后等待回答。请求已经完整时可以零问题推进。

Matt 已安装且允许 model invocation 的 `/grilling` primitive 是决策访谈的可选加速器。因为它允许 model reach，workflow 可以运行 `/grilling` skill，并遵守一次一个问题与共同理解确认的规则。缺少它不会阻断 workflow，也不会触发安装。

会产出 artifact 的 primitive 不是只读捷径：`/domain-modeling` 会写 glossary 或 ADR，`/research` 会写 Markdown 报告，`/prototype` 会写一次性代码。只有用户原始请求已经分别授权该精确 artifact 与范围时，workflow 才能调用其中之一；否则必须直接调查并报告仍无法消除的不确定性。若未解决的决策地图过大，无法在一个 context 内可靠完成，宿主不得悄悄持久化它；应返回范围明确的已决/未决地图，并建议用户重新明确调用 workflow，或在已安装时由用户单独调用 `/wayfinder`。

## Matt Pocock 方法的映射

Matt 的 `/grill-me`、`/grill-with-docs`、`/wayfinder`、`/to-spec`、`/to-tickets`、`/implement` 与 `/ask-matt` 前门都是 user-invoked，另一个 skill 不得触发它们。因此 Slipway 内化其中有用的方法纪律，同时把这些原始命令保留为可选 wizard 路径。无论 `code-review` 的 invocation 设置如何，本 workflow 都不调用它，因为 handoff 后的执行与 Review 只由 Slipway Run 拥有。

| Matt 方法或产物 | 在 Slipway workflow 中的含义 |
| --- | --- |
| `grill-me` / `grill-with-docs` | 已安装时可以复用其 model-invocable `/grilling` primitive；前门仍只能由人调用，文档写入仍需单独授权 |
| `wayfinder` 的 destination 或 Issue map | 需要多个独立交付时形成一个 Objective；跨 session 的持久地图仍是外部 workflow，必须单独调用 |
| `to-spec` 的 spec/PRD | 规划输入，规范化为下述 Change 或 Objective 章节 |
| `to-tickets` 的 tracer bullet | Objective 中的暂定 Changes，再由明确调用的 `slipway-decompose` 创建 marker-valid 子 Change |
| `implement` / `code-review` | handoff 前不使用；之后由 Slipway Run 拥有 Implement 与建议性的 Review |

`slipway-workflow` 永远不会调用 `/grill-me`、`/grill-with-docs`、`/wayfinder`、`/to-spec`、`/to-tickets`、`/implement`、`/code-review` 或 `/ask-matt`。用户仍可自行逐项调用其中的 user-only 命令，把它们作为另一条多命令 wizard 路径。

## 选择正确的草稿层级

**Change** 是一个可以独立交付、验证和回退的结果；它使仓库保持安全状态，并大致适合一个全新 Agent context。它有五个可独立寻址的角色：

- Outcome
- Requirements
- Acceptance examples
- Constraints
- Non-goals

**Objective** 是必然需要多个独立有用交付的更大目标，只用于规划，包含：

- Problem
- Outcome
- Requirements
- Shared constraints
- Non-goals
- Changes，包括暂定的 tracer-bullet 切片与阻塞边

只有自包含且 marker-valid 的 `change/v2` Issue 可以启动 issue-backed Run，Objective 不可以。纯调查工作应是 `kind:research` Change，交付有证据支持的结论，而不是代码。

## 发布与来源 handoff

Workflow 返回完整草稿及预期发布形态后就停止；它不会声称已经生成“获确认的精确发布计划”。仓库重新获取、operation identity、精确 body 与 digest、关系 revision、preview，以及对该计划的一次当前确认，都只由 `slipway-propose` 拥有。详见 [GitHub Issue 工作流](github-issues.md)。

普通 spec 或 tracker Issue——包括 `to-spec` 或 `to-tickets` 创建的 Issue——只是非权威规划输入，不能直接交给 Run。Change 发布后，宿主报告 canonical URL 与编号。随后用户另行明确调用 `slipway-run`；宿主获取并 attest 该 Change，构造临时 Source Bundle envelope，再用 `--source-file` 交给本地 CLI。CLI 不联网获取 GitHub，裸 Issue 编号也不是 CLI 来源。

对于很小、私密、紧急、离线或明确不跟踪的任务，workflow 可以改为建议用户另行启动 ad-hoc `slipway-run`，使用已经澄清的 goal。这是明确的来源选择，不是让 workflow 越权启动执行的捷径。

## “自主跑下去”的准确含义

Issue-backed Run 应使用 `1..1000` 范围内经过考虑的 budget。Run 在固定 Requirements 与可用 budget 内一次推进一个 Action，直到 `paused`、`stopped` 或 `ended`。Run 内 Clarify 仍可能因真正的人类决策暂停；高质量草稿会减少这种情况，但不能禁用它。`budget_exhausted` 是正常、可恢复的暂停。Resume 时明确传入 `--budget N` 会把 remaining budget 替换为 `N`；省略时保留正余额，零余额补到 `max(initial_budget, 3)`。只有 mutation 真正恢复 Run 时才应用替换。

Review 只读且仅提供建议。Finding 不会自动建立 Implement/re-review 循环；`ended` 只表示自动 Action 队列为空，并不证明正确、完成或可发布。用户可以把 finding 放入新 Change，或对同一 Change 启动另一条 Run。详见 [Run、恢复与隐私](runs-and-recovery.md)。
