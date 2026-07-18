# 从想法到 Run 的工作流

`slipway-workflow` 是一个必须由用户明确调用的宿主侧协调能力，编排的是 Slipway Issue 工作流定义的功能。它可以从粗略想法、现有 Objective 或 Change、以及已有 Run 出发，判断下一项仍由用户拥有的 capability，或者明确给出“不再行动”的结果。它不会把宿主已安装的 skills 变成通用流水线，不增加持久 workflow state，也不复制 Run scheduler。

本指南使用 capability 名称，不限定各宿主的调用语法；具体入口见[宿主适配器](../reference/adapters.md)。

## 一次调用授权什么

Workflow 可以检查当前事实、组织决策访谈，并在当前阶段需要时综合 work-item 草稿，但不能跨越下一项 capability 边界，也不能在用户选择不再行动时发明一个边界：

| 功能 | AI 在本次调用中可自主完成 | 仍由用户拥有 |
| --- | --- | --- |
| 定位与草拟 | 读取仓库和生命周期事实、组织访谈、选择 Change 或 Objective、综合完整草稿 | 真正的产品或风险决策 |
| 发布 | 解释预期发布形态并给出路由 | 单独调用 `slipway-propose`，并对其精确外部写入计划做当前确认 |
| 拆分 | 解释 Objective 为何需要自包含子 Change 并给出路由 | 单独调用 `slipway-decompose`，并对其精确 operation plan 做当前确认 |
| 执行或恢复 | 解释 source、Run 与 budget 路由 | 单独调用 `slipway-run` |

这里的“无状态”只表示 workflow 不创建 Slipway Run、journal 或跨阶段 cursor。对话、文档、tracker artifact、prototype 与代码修改仍然是状态或副作用。Workflow 本身只读；已有的 external/unmanaged artifact 可以作为非权威规划输入，但合法的 managed Objective、Change 与 Run record 保留契约定义的路由和 source authority。创建或修改任何 artifact 都必须是另一项明确 detour。

## 最短合法路径

Workflow 根据观察到的起点跳过不适用的阶段：

| 起点 | 立即交接的 owner 或 terminal outcome | 后续条件路径 |
| --- | --- | --- |
| 粗略想法、已澄清对话、spec、PRD、map 或 ticket list | 完成 Change 或 Objective 草稿后调用 `slipway-propose` | 已发布 Change → `slipway-run`；已发布 Objective → `slipway-decompose` → 用户选定子 Change → `slipway-run` |
| 明确的纯决策澄清请求，终点是 bounded summary 而不是草稿 | `slipway-clarify` | Clarify 保持无状态，不物化也不执行 |
| 结构合法的 Objective | `slipway-decompose` | 成功发布的子 Change → `slipway-run` |
| 结构合法、自包含的 `change/v2` Issue | `slipway-run` | Run 拥有 Orient、必要时 Clarify、Implement、建议性 Review 与 Summary |
| 明确为私密、很小、紧急、离线或有意不跟踪的 bounded goal | `slipway-run` | 在已澄清 goal 上启动新的 ad-hoc Run，不隐含 Issue source |
| active Run | `slipway-run` | 使用当前精确 Action 与 submit/skip variants；stop 使用公开命令，take-over/reorder 先 stop 再交还控制权 |
| paused 或 stopped Run | `slipway-run` | 使用当前结构化 recovery variant，不从 prose 重建 |
| failed、partial 或 ambiguous 的 Propose/Decompose publication | 原始 `slipway-propose` 或 `slipway-decompose` owner | 返回所有可用 receipt/operation/item/revision facts；由 owner 决定按同 receipt 对账，还是按契约重新 preview 与确认 |
| ended Run 或 Review findings | 根据 provenance 做一个包含“不再行动”的真正决定 | 停止且不点名 capability；新 tracked scope → Propose；同一已接受 Change scope → fresh-fetch/attest 后启动新的 issue-backed Run；没有 Change → 新 ad-hoc Run |

只有用户选择继续时，Workflow 才点名一项立即执行的下一 capability，而且不会调用它。对于 ended work、advisory finding 或用户放弃的 publication attempt，“不再进行 Slipway action”是合法 terminal outcome；workflow 如实报告剩余状态，不发明下一 capability。失败、部分成功或歧义的发布交回原 Propose/Decompose owner，不能推进到 Decompose 或 Run。Workflow 不盲重试、重启或发明 operation；owner 可以使用同一 receipt，或按契约重新 preview 并取得当前确认。

当用户要求的终点只是 bounded decision summary，而不是草稿或发布时，可以使用 standalone `slipway-clarify`；workflow 不把它强行插入草拟路径。只有当用户明确需要没有 Run attribution 和固定 Issue source 的 standalone 路径时，才使用直接的 `slipway-implement` 或 `slipway-review`；它们不是绕过托管 Change-to-Run 路径的捷径。Active Run 已经拥有自己的 Action loop：只有 submit 与 skip 是 Action variants；stop 使用公开的 `slipway stop`，take-over/reorder 则先 stop 再交还控制权。

Ended Run 是 terminal，绝不 resume。Finding 是 advisory，用户可以接受并选择不再行动。若用户选择新的 issue-backed attempt，必须重新 fetch/attest canonical Change，不能把 ended Run 的 pinned snapshot 当作新 source evidence。没有 Change 的 ended ad-hoc Run 或 standalone Review finding，只有在用户选择重试时才在已澄清 goal 上启动新的 ad-hoc Run；新的或改变后的 tracked scope 必须先走 Propose。

## 决策访谈，而不是 skill 目录路由

Workflow 先检查当前 Git 状态、相关代码与测试、以及仓库的验证约定。能查明的事实不拿来询问用户。确有只能由人决定的问题时，每次只问一个，给出推荐、理由、替代方案与取舍；完整请求无需访谈。

Workflow 本身自包含。已经安装且允许 model invocation 的 `/grilling` 是唯一可选的外部加速器；使用时仍遵守一次一个问题与共同理解确认。该确认只确认草稿所使用的理解，并不授予发布、拆分、实现或 Run 权限。缺少 `/grilling` 不会阻断 workflow，也不会触发安装。

Workflow 不发现、排序或调用宿主的其他 skills。User-only 前门仍只能由人调用，外部 implementation/review skill 也不能替代 Slipway Run。若确实需要单独的 ADR、报告、prototype 或持久 wayfinding map，workflow 只解释 detour 后停止；用户可以另行明确调用对应工具，之后再把产物作为非权威规划输入带回来。

## 选择正确的 work-item 层级

**Change** 是一个可以独立交付、验证和回退的结果；它使仓库保持安全状态，并大致适合一个全新 Agent context。它有五个可独立寻址的角色：

- **Outcome**
- **Requirements** — 优先描述行为和契约；当精确路径、格式或示例本身就是必要约束时应保留
- **Acceptance examples** — 必须可客观验证；面向用户的工作优先验证外部行为，refactor/maintenance 可以同时验证 preserved behavior 与 measurable internal outcome，research 则验证应交付的证据与结论
- **Constraints**
- **Non-goals**

**Objective** 必然需要多个独立有用的交付，只用于规划，包含：

- **Problem**
- **Outcome**
- **Requirements**
- **Shared constraints**
- **Non-goals**
- **Changes**，包括暂定 tracer-bullet slices 与 blocker edges

只有结构合法、自包含、manifest-addressed chapters 通过 source validation 的 `change/v2` Issue 可以启动 issue-backed Run。Objective 永远不可执行。纯调查是 `kind:research` Change，交付有证据支持的结论；后续代码使用另一个 Change。

## 发布与 source handoff

Workflow 返回完整草稿和预期发布形态，而不是 Propose 的 approved publication plan。仓库 refetch、operation/item identity、精确 body/digest、relation revision、preview、reconciliation 与该计划的当前确认都只由 `slipway-propose` 拥有；`slipway-decompose` 拥有发布子 Change 的对应 operation。

Change 成功发布后，宿主报告 canonical URL 与编号。Objective 成功拆分后，宿主报告每个子 Change URL、解释建议性的 unblocked frontier，并在保留用户选择权的前提下推荐一个 Change。只有到这一步，用户才对选中的 Change 明确调用 `slipway-run`。

宿主获取并 attest 该精确 Change，构造临时 Source Bundle envelope，再用 `--source-file` 交给本地 CLI。CLI 不联网获取 GitHub，裸 Issue 编号也不是 CLI source。

启动新 Run 时，若用户没有覆盖初始 budget，宿主说明并使用契约默认值 `8`；明确覆盖值可以是 `1..1000`。推荐更大值必须说明理由，并且绝不承诺完成。Resume 使用下文独立的 remaining-budget 规则。对于很小、私密、紧急、离线或明确不跟踪的工作，workflow 可以改为路由到使用已澄清 goal 的显式 ad-hoc `slipway-run`。

Workflow 不增加额外的治理 gate。每次外部写入 operation 继续使用该 operation 范围内的当前确认，Run start 仍需单独明确授权。

## “自主执行”的准确含义

Run 明确启动后，在固定 Requirements 与 budget 内一次推进一个 Action，直到 `paused`、`stopped` 或 `ended`。Run Clarify 仍可能为真正的人类决定暂停。`budget_exhausted` 是正常、可恢复的暂停。Resume 明确传入 `--budget N` 会替换余额；省略时保留正余额，零余额只在 mutation 真正恢复 Run 时补到 `max(initial_budget, 3)`。

Review 只读且仅提供建议。Finding 不会自动建立 Implement/re-review loop；`ended` 只表示自动 Action queue 为空，并不证明正确、完成或可发布。
