> 本正文为 v2，supersedes 此 Issue 之前的 v1 source/protocol 章节（change/v1 → change/v2 manifest-addressed、contract_version 1 → 2）。v2 与 ADR-0001、machine-protocol.schema.json 一致。

# 产品定义

> Slipway 是一个由用户显式启动、Issue 驱动但不被 GitHub 阻塞、CLI 调度、可中断恢复的 AI coding 软自动驾驶。

核心工作流：

```text
Objective Issue（可选父级）
        ↓ native sub-issues
Change Issue（唯一可执行工作单元）
        ↓
Run（一次执行尝试）
        ↓
Code + Tests + User Docs
```

总体产品设计、功能地图或其他上游规划不属于 Slipway 的强制 artifact。用户可以通过任意外部框架、普通文档、Issue 或手工过程形成 Objective；Slipway 的责任边界从 Objective/Change Issue 开始。

---

# 1. 产品哲学

## 1.1 Requirements-only

Slipway 不维护 Spec、Delta Spec 或“当前系统全部有效需求”的永久目录。

正式原则：

> Requirements are temporary delivery contracts, not a permanent model of the system.

即：

- 开放 Issue 描述下一次希望发生的变化；
- Run 固定并执行某个版本的 Change Issue；
- 交付后，代码、测试、用户文档、CI/policy 和运行行为接管当前事实；
- 已关闭 Issue、关联 PR/commit 与 Run 总结保留历史原因，但不成为当前系统的完整规格；
- GitHub 的 `closed`、Project `Done`、PR `merged`、Run `ended` 和 deployment 是不同事实。

如果未来确实需要合规级 living requirements，应作为新产品决策显式引入，不能把它偷偷重建在 GitHub 字段或新文件中。

## 1.2 Issue-first，不是 Issue-gated

非临时工作默认从 Change Issue 开始，但 CLI 保留：

```bash
slipway run "<ad-hoc goal>"
```

用于微小修改、GitHub 不可用、私密安全工作、紧急修复或用户明确不想创建 Issue 的场景。外部服务不能成为新的 hard gate。

## 1.3 用户拥有过程

- 只有用户显式调用 Slipway 入口能力或 CLI 才开始；一次显式 Run 授权预算和 Requirements 内的自动 Action 循环；
- 用户可以随时 skip、stop、resume 或接管；
- 普通实现不重复询问授权；
- source amendment、真正的人类决策、环境不可用和破坏性操作必须暂停；
- 破坏性确认是 scope-bound 的 trusted-host attestation，不声称密码学证明人类存在；
- 测试失败、未运行测试、Review finding、脏工作树、缺少 ADR、label 或 Issue 状态都不得成为 gate；
- `ended` 只表示自动 Action queue 已空，不是完成、正确、可发布或已交付认证。

---

# 2. 正交模型：Level、Kind、Requirements、Status

以下四个维度禁止混用。

| 维度 | 值 | 所有者 |
| --- | --- | --- |
| Level | `objective` / `change` | Issue body marker（唯一权威）；labels/title 只是 projection |
| Kind | `feature` / `bug` / `refactor` / `maintenance` / `research` / `docs` | repository label |
| Requirements | Outcome、Requirements、Acceptance examples、Constraints、Non-goals | Issue body |
| Status | Inbox、Clarifying、Ready、In progress、Done 等 | 人类/可选外部视图，不参与 Slipway 路由 |

Level 与 Kind 的笛卡尔积均可合法使用；下列只是代表性示例，不是穷举：

```text
Objective + feature / bug / refactor / maintenance / research / docs
Change    + feature / bug / refactor / maintenance / research / docs
```

Requirement 只是 Objective/Change 的内容表达方式，不是颗粒度，也不是 GitHub Issue Type。

---

# 3. Objective Issue

Objective 只在目标明显需要多个独立 Change 时创建。小型 feature、bug 或 refactor 直接创建 Change，不制造空父级。

## 3.1 语义与继承边界

Objective 表达一个需要多个交付才能实现的完整结果：

- 可以是大功能、一组相关 bug 修复、大型重构、兼容性迁移或研究计划；
- 不直接交给 Implement，不作为 Run primary source；
- 不带 `ready-for-agent`；
- 保存用于拆分的共同目标、需求、约束和非目标；
- 不复制每个子 Change 的完整 acceptance examples；
- 不构成运行时继承链，也不参与 Change source 的有效 Requirements 计算。

**Change 必须自包含。** `decompose` 在创建子 Change 时，将所有适用于该子项的 Objective 需求和约束物化到 Change 正文。一个 Change 被标记为 ready 前，其正文必须包含独立执行所需的全部有效 Requirements；Run 不得依赖实时读取父 Objective 才能理解任务。

Objective 后续编辑不会静默改变已有 Change。若共同需求改变，必须由人类手工 amendment 受影响的开放 Change，或由用户显式再次调用 `decompose` 进入 amendment mode：宿主先给出逐个开放子 Change 的 Requirements diff、expected source revision 与 publication plan，用户批准后才逐项 PATCH；并发变更、适用范围不清或任何 item 失败时暂停对账，不自动扩散。已关闭/已交付 Change 不重写，改为创建新的 superseding Change。该流程是用户驱动的显式维护便利，不是继承、后台同步或 readiness gate。父子 Kind 相互独立，子 Change 不继承父 Objective 的 `kind:*`。

## 3.2 Body marker

```html
<!-- slipway-level: objective/v1 -->
```

Marker 必须是正文第一个非空行、位于任何 code fence 之外且逐字匹配；其他位置出现的示例文本不具有 Level 语义。

## 3.3 标题与标签

```text
Title: [Objective] <outcome>
Labels: level:objective + exactly one kind:*
```

## 3.4 正文模板

```markdown
<!-- slipway-level: objective/v1 -->

## Problem

## Outcome

## Requirements

## Shared constraints

## Non-goals

## Changes

由 GitHub native sub-issues 表达。
```

---

# 4. Change Issue

Change 是唯一可以作为 **issue-backed primary source** 绑定 Run 的工作单元；显式 ad-hoc Run 仍可不使用 Issue。

## 4.1 颗粒度与自包含不变量

一个 Change：

1. 只有一个连贯、可观察的结果；
2. 可以独立 merge 或交付；
3. 可以独立验证和回滚；
4. 完成后仓库仍处于有意义且安全的状态；
5. 大致适合一个 fresh Agent context；
6. 可以跨 UI、API、存储与测试形成 vertical slice；
7. 若其中两个工作可以独立交付，则应拆成两个 Change；
8. 实施步骤只放普通 checklist，只有可独立交付的步骤才升级为另一个 Change；
9. 包含执行所需的全部有效 Requirements，不依赖父 Objective 的隐式继承；
10. 普通讨论 comments 不是 Requirement authority；评论中的决定只有被发布成 replacement chapter 并进入新 manifest 后才属于下一 snapshot。

## 4.2 Body marker

````markdown
<!-- slipway-level: change/v2 -->

```slipway-manifest
{
  "manifest_version": 2,
  "profile": "change/v2",
  "sections": [
    {
      "key": "outcome",
      "role": "outcome",
      "title": "Outcome",
      "comment_node_id": "IC_outcome",
      "comment_database_id": 101,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "requirements",
      "role": "requirements",
      "title": "Requirements",
      "comment_node_id": "IC_requirements",
      "comment_database_id": 102,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "acceptance-examples",
      "role": "acceptance_examples",
      "title": "Acceptance examples",
      "comment_node_id": "IC_acceptance_examples",
      "comment_database_id": 103,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "constraints",
      "role": "constraints",
      "title": "Constraints",
      "comment_node_id": "IC_constraints",
      "comment_database_id": 104,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "non-goals",
      "role": "non_goals",
      "title": "Non-goals",
      "comment_node_id": "IC_non_goals",
      "comment_database_id": 105,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    }
  ]
}
```
````

一个托管 Change 的正文必须以唯一受支持的 `change/v2` level marker 作为第一个非空行，下一非空 block 必须是唯一严格 `slipway-manifest` JSON fence，且 code fence 外不得再出现其他 `slipway-level` marker。Objective、v1、未知或冲突 marker 不能作为 Change Run source，必须拒绝并要求人类修正。

## 4.3 标题与标签

```text
Title: [Change] <independently deliverable outcome>
Labels: level:change + exactly one kind:*
Optional advisory label: ready-for-agent
```

`ready-for-agent` 只是搜索提示，永远不是 `slipway run` 的 gate 或路由输入。

## 4.4 正文模板

````markdown
<!-- slipway-level: change/v2 -->

```slipway-manifest
{
  "manifest_version": 2,
  "profile": "change/v2",
  "sections": [
    {
      "key": "outcome",
      "role": "outcome",
      "title": "Outcome",
      "comment_node_id": "IC_outcome",
      "comment_database_id": 101,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "requirements",
      "role": "requirements",
      "title": "Requirements",
      "comment_node_id": "IC_requirements",
      "comment_database_id": 102,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "acceptance-examples",
      "role": "acceptance_examples",
      "title": "Acceptance examples",
      "comment_node_id": "IC_acceptance_examples",
      "comment_database_id": 103,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "constraints",
      "role": "constraints",
      "title": "Constraints",
      "comment_node_id": "IC_constraints",
      "comment_database_id": 104,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    },
    {
      "key": "non-goals",
      "role": "non_goals",
      "title": "Non-goals",
      "comment_node_id": "IC_non_goals",
      "comment_database_id": 105,
      "body_sha256": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
    }
  ]
}
```
````

每条 manifest entry 引用一个 GitHub Issue comment；该 comment 的第一个非空行必须逐字匹配：

```html
<!-- slipway-section:v1 key=KEY -->
```

其后是该 chapter 的 exact normalized Markdown payload。Manifest array 是章节顺序和唯一 source HEAD；每项把 stable key、role、title 绑定到 GitHub comment GraphQL node ID、REST database ID hint 和完整 comment body digest。`change/v2` profile 至少包含 `outcome|requirements|acceptance_examples|constraints|non_goals` 五类 role，但每类可拆成多个独立可寻址 chapter。

Parser 不从 comment 顺序、时间或 marker-like 普通评论发现 authority，不扫描未被 manifest 引用的 comments，不对 payload 做模型总结或 Unicode normalization。Accepted chapter 不原地编辑；修改创建 replacement comment，并在最后一步提交新 manifest。不得让模型重新总结后再声称等价。Relationships 使用 GitHub native parent/sub-issue 和 blocked-by；正文普通链接仅作为无法建立原生关系时的 fallback。

## 4.5 Bug、refactor 与 research

它们不是额外颗粒度。

- 小 bug：一个 `level:change + kind:bug`；
- 大 bug 修复计划：一个 `level:objective + kind:bug`，下面拆 Changes；
- 小重构：一个 `level:change + kind:refactor`；
- 大重构：一个 `level:objective + kind:refactor`，按 expand/migrate/contract 等可交付步骤拆分；
- 调查：`kind:research`，交付物是有证据的结论；若结论要求改代码，再创建 Change，不在调查中静默扩大范围。

纯重构或恢复已有行为时不伪造外部行为变化。正文应写 preserved behavior、measurable internal outcome 与 validation。

---

# 5. GitHub：非 Organization repository 的前提

本设计面向 GitHub.com 上启用了 Issues 的普通个人或 Organization repository，但不得依赖 Organization Issue Types、Organization Issue Fields 或 GitHub Project。读取需要 Issues read；创建、打 label 和建立关系至少需要目标仓库允许的 triage/write 权限。

## 5.1 身份权威与 projection

优先级固定为：

1. Issue body marker：Level 的机器权威；
2. repository labels：可修复的搜索/navigation projection；
3. `[Objective]` / `[Change]` 标题前缀：可修复的人类提示。

必须提供以下 repository labels：

```text
level:objective
level:change
kind:feature
kind:bug
kind:refactor
kind:maintenance
kind:research
kind:docs
ready-for-agent
```

托管 Issue 应恰好拥有一个 `level:*` 与一个 `kind:*`，但 GitHub 本身不强制互斥。marker 与 labels/title 冲突时，marker 决定 Level，宿主报告 drift；只有用户确认后才修复外部 projection。缺少或错误 label 不得阻止一个 marker-valid Change 启动 Run。

现有普通 Issue 若没有 marker，不能被宿主静默解释为 Change。由于 GitHub body PATCH 没有原子 compare-and-swap，Slipway 不自动原位重写其正文；`propose` 应展示三种明确选择：用户手工应用规范化正文、经确认创建一个链接原 Issue 的独立托管 Change、或把 bounded summary 作为 ad-hoc Run。任何选择都不得悄悄制造重复工作项。

## 5.2 原生关系与限制

- Objective → Change：GitHub native sub-issues；
- Change → Change：GitHub native blocked-by dependencies；
- 只使用一层 Objective → Change，不构建 Jira 式深层 hierarchy；
- 每个父 Issue 最多 100 个 sub-issues，GitHub 最多八层；Slipway 到达 100 个时必须停止发布并建议重新划分 Objective；
- 每个 Issue 的 blocking 与 blocked-by 每个方向最多 50 个；到达限制时停止并报告，不降级成不可见的文本图；
- GitHub `closed` 不证明 blocker 已实际交付；宿主可以解释 frontier，但不自动锁定，用户保留覆盖权；
- 添加 approved sub-issues/dependencies 是 `decompose` 获批的关系修改，不等于授权编辑 Objective body 或关闭 Objective。

## 5.3 GitHub Project

GitHub Project 完全可选：

- Slipway 不读取 Project ID、field ID、Status 或 iteration；
- 不要求创建 Project；
- Project 可作为 table/board/roadmap 视图加入，不影响 Issue 身份或 Run journal；
- Project 字段不得成为 Requirements、gate、freshness 或完成认证来源。

## 5.4 工具兼容与身份变化

- 一等 `gh issue create/edit/view` parent、sub-issue 和 dependency flags 需要 `gh >= 2.94.0`；宿主必须探测版本，不满足时使用官方 REST API fallback 或报告 `environment_unavailable`；
- 不得把 token 写入 URL、Action、Issue 或 journal；
- provider identity 使用 GitHub repository node ID + issue node ID；number、title 和 URL 不是稳定身份；
- transfer 会改变 repository、number 和 canonical URL，旧 URL 可能重定向。宿主只跟随同一 `github.com` 信任边界内的重定向，重新读取身份、labels、parent 与 dependencies，并把旧 URL 作为 alias；
- `404` 也可能表示权限丢失、repository 变私有或 Issue 消失；统一作为 `source_unavailable`，不另设来源消失状态。

## 5.5 外部写入、部分成功与对账

GitHub Create Issue API 没有 exactly-once/idempotency key，Issue body PATCH 也没有可依赖的原子 compare-and-swap。Slipway 不得承诺绝不重复或无竞态更新，而必须实现可恢复对账：

1. 调查仓库、现有 Issues 和将被增加关系的 current revisions；
2. 在外写前生成 approved publication plan：operation UUID、每个 item UUID、目标 repository、canonical body digest、labels、parent/dependencies、预期 current revisions；
3. 展示完整草稿和 plan，获得用户对本次精确外部写入的显式确认；
4. 每个新 Issue 在 level marker 后写入两个有类型 marker：

   ```html
   <!-- slipway-publication-operation: <operation-uuid> -->
   <!-- slipway-publication-item: <item-uuid> -->
   ```

5. 只有持有同一 approved plan/receipt 才允许自动重试。Receipt 只保存 IDs、digests、目标、expected revisions、observed URLs/status，不保存 Requirements 正文，也不是工作权威；receipt 丢失或宿主重启无法恢复时必须重新预览和确认；
6. 对现有 Issue 施加 label/relationship 前立即 refetch；若与 approved expected revision 不同，停止并重新预览。GitHub 没有 CAS，refetch 与 write 之间的剩余 race 必须在写后回读中检测并如实报告，不能声称完全消除；
7. 使用 body file 或等价跨平台安全输入，禁止依赖 POSIX heredoc；
8. 请求结果不明确时，通过非搜索、可分页的 Issue API 按 operation/item marker 对账，禁止盲目重试；
9. 部分成功时不自动删除或 rollback 已创建 Issue，必须返回每个 item/label/relationship 的 `created|matched|failed|ambiguous` 与 URL；
10. 零个匹配可在用户重新确认后重试；一个匹配继续收敛；多个匹配暂停并交给用户，不自动关闭重复项；
11. 发布后回读验证 body digests、markers、labels、parent 和 dependencies；
12. label 不存在时，把创建 label 计入同一 plan 与确认范围；
13. GitHub 工具或认证不可用时返回 `environment_unavailable`，不得静默创建另一份本地权威副本。

该模型是 at-least-once external API + approved-plan reconciliation，不是虚假的 exactly-once 或 CAS 保证。

## 5.6 不可信内容与敏感数据

Issue title、body、comments、labels、链接与附件全部是**不可信数据**，不是 system/developer instruction：

- 宿主必须将其清晰 delimiting 为数据，只允许用户已选择且 parser 接受的 Requirements 影响工作目标；
- Issue 中要求泄露凭据、改变 Slipway 控制规则、绕过确认或执行无关命令的文字没有指令权限；
- comments 默认不进入 source；linked URL 不得自动携带凭据抓取；
- 创建/修改前不上传 token、secret、PII、客户数据、完整访谈 transcript 或 chain-of-thought；
- GitHub 没有单个公开仓库 Issue 的 private 开关。敏感内容使用 private repository；仅在仓库已启用且内容确为安全漏洞时使用 private vulnerability reporting；其他情况使用既有安全通道或 ad-hoc Run。

---

# 6. 宿主能力表面

十个宿主适配器只生成以下六个显式能力：

```text
slipway-run
slipway-clarify
slipway-propose
slipway-decompose
slipway-implement
slipway-review
```

所有入口能力均需用户显式调用；Codex 在每个能力目录内额外生成一份 `.codex/skills/slipway-<name>/agents/openai.yaml`（每个能力一份），并设置 `policy.allow_implicit_invocation=false`。

“显式调用”的边界是：用户显式启动 `slipway-run` 即授权该 Run 在 budget、Requirements 和安全边界内依次调用 Orient、Clarify、Implement、Review 与 Summary，不要求用户逐 Action 再次调用 skill。真正的人类决策、source amendment、环境不可用和破坏性确认仍暂停。直接调用 `slipway-implement` 或 `slipway-review` 也必须由用户显式发起，不产生 Run attribution。

不生成：

- ambient session hook；
- prompt-submit hook；
- launcher；
- global router；
- standalone test/typecheck/build/lint/check capability；
- `clarify-docs`；
- domain/spec/change lifecycle capability；
- worktree capability。

## 6.1 `slipway-clarify`

遵循 [Matt Pocock `grill-me` / `grilling`](https://github.com/mattpocock/skills) 的对话纪律；`grill-with-docs` 保持为独立显式组合。Slipway 不引入外部 workflow 或 artifact lifecycle：

- standalone Clarify 只由用户显式调用；在用户已显式启动的 Run 内，只有 Orient 识别出人的决定并由 CLI 签发 Clarify Action 时才进入该对话，不要求用户重复启动，也不因普通聊天 ambient 触发；
- 沿 design tree 按依赖顺序逐个消解决定；一次只问一个真正的人类决策，并等待答案；
- 每题包含推荐答案、理由、替代方案和权衡；
- 能从代码、测试、文档或 Git 得出的事实由 Agent 先查明，不转嫁给用户；决定始终交给人类，不由 Agent 自问自答；
- 完整请求允许零问题。若本次 grilling 新增或改变了执行理解，进入 Implement 前必须再得到用户对当前共同理解的明确确认；若无需 grilling，原始显式 Run 请求已足够，不增加重复确认；
- 该确认只是当前会话从澄清转入执行的 consent boundary，不是 readiness、质量、Issue 状态或交付 gate；
- 默认无状态，不写文件、不创建 Issue；文档化是另一个显式能力，不从 Clarify 偷渡；
- 用户要求 wrap up 时立即停止并总结，不 materialize、不执行；不保存完整 transcript 或已被后续答案推翻的内容。

这里的 “relentless” 指在用户愿意继续时追到依赖决策清楚，不表示强迫穷尽、拒绝停止或扩大 Change 颗粒度。对话深度、Requirements 文体与 Objective/Change/Run 颗粒度保持正交。

## 6.2 `slipway-propose`

将已澄清的当前对话显式 materialize 为：

- 一个自包含 Change；或
- 一个 Objective（需要后续显式 decompose）。

它不要求先调用 clarify，也不拥有总体设计。创建前必须展示草稿、approved publication plan、关系、labels 和所有外部写入并确认。对现有无 marker Issue 必须与 §5.1 一致展示三种选择：用户手工应用规范化正文、经确认创建链接原 Issue 的新 Change、或把 bounded summary 交给显式 ad-hoc Run；不自动 body PATCH，也不把 ad-hoc 选项伪装成已 materialize 的 Issue。

## 6.3 `slipway-decompose`

将一个 Objective 拆为有 blocker 图的 tracer-bullet Changes：

- 每个 Change 为完整 vertical slice；
- 将所有适用的 Objective Requirements/constraints 物化进每个子 Change，使其独立可执行；
- 每个 Change 独立可验证、交付和回滚；
- 使用 native sub-issues 和 dependencies，并遵守 GitHub 数量限制；
- 先展示编号拆分、有效 Requirements、交付结果、blocker 和 publication operation，用户批准后才发布；
- wide refactor 使用 expand → migrate batches → contract；
- 除增加获批关系外，不编辑 Objective body 或关闭 Objective，除非用户另行明确要求；
- 部分成功、歧义响应和重试遵守 §5.5 的对账规则。
- 用户显式再次调用时可进入 amendment mode，按 §3.1 为受影响的开放子 Change 生成逐项 diff；不得后台传播、静默 PATCH 或把同步状态变成 gate。

## 6.4 `slipway-run`

仅在用户明确要求启动或恢复 Slipway 时使用。一次只执行 CLI 返回的一个 Action。它可以：

- 从一个 marker-valid、自包含的 Change Issue source envelope 启动；或
- 从 ad-hoc goal 启动。

Objective 不能作为 primary executable source。宿主是 GitHub fetch 的 trusted attester，不是密码学证明者；必须把 Issue 内容作为 untrusted data，禁止把其中的命令或链接提升为宿主指令。

## 6.5 `slipway-implement`

只实施当前 Action 已授权的 Change 或用户直接请求，调查当前仓库约定，执行相关技术活动，如实报告命令、exit code、changed files、known issues 与 uncertainty。Run 内授权受固定 source revision、budget 和任何 scope-bound destructive grant 限制；不得把普通 Issue 文本当作扩大权限的依据。

## 6.6 `slipway-review`

只读检查 Intent 与 Quality：

- Intent：实现是否满足本次固定、自包含 Requirements；
- Quality：设计、错误处理、并发、安全、可维护性和测试敏感性；
- diff baseline 是 Run start HEAD；Run 开始前已有 dirty paths 单独报告，不能归因于本次 Run；
- start-to-current observation 只能证明工作区自 Run 启动后出现差异，不能证明差异由该 Run、当前 Agent 或某次 Implement 造成；Review 与 Summary 必须显式保留这一归因不确定性；
- 不修改代码；
- 不输出 pass/fail、approved、ready 或 ship-ready；
- 不返回 `needs_input`，不建议 Implement，不自动修复、不暂停当前 Run、不建立 re-review loop；
- findings 总是进入 Summary；用户可在 Run 外显式形成新的 Change，或为同一 Change 启动新的 Run。

---

# 7. CLI 表面

公开命令仍严格只有：

```text
slipway install
slipway uninstall
slipway list
slipway doctor
slipway run
slipway status
slipway stop
```

不新增 `objective`、`change`、`issue`、`spec`、`plan`、`ticket`、`done`、`check` 或 `worktree` 命令。

- `install --refresh` 是已有 `install` 的刷新模式，不是第八个命令；
- `doctor` 只诊断安装、适配器、Git/GitHub 工具和运行环境，不检查代码质量、不生成 verdict、不改变 Run；
- `list` 只列宿主适配面和安装状态；`status` 只列 Run/source/recovery 状态；
- Implement 与 Review 是宿主能力，不是顶层 CLI 生命周期命令。
- 所有公开或隐藏 machine JSON 输出——包括 install/uninstall/list/doctor、run start、`_machine submit/answer/skip/resume/material`、status 与 stop——均属于版本化稳定契约；字段语义只按明确的 contract version 演进。人类可读 prose 不是 JSON 字段的兼容替代。

## 7.1 启动

Ad-hoc：

```bash
slipway run "<goal>" --budget 8 [--no-review] --json
```

Issue-bound：

```bash
slipway run "<bounded goal>" \
  --source-file <raw-github-change-envelope.json> \
  --budget 8 [--no-review] --json
```

新 Run 的 `--budget` 默认值为 `8`；显式提供时必须在 `1..1000` 范围内（最小 `1`、最大 `1000`）。

`source-file` 是 trusted host 从 GitHub 获取的临时 raw envelope。CLI 安全读取一次、确定性验证/解析、计算 revision，并只把 accepted snapshot 写入 journal；后续不依赖该临时文件存在。

CLI 本身不持有 GitHub token、不调用模型、不实现 GitHub/Project provider，也不把 source-file 声称为密码学可验证的 GitHub 证明。

## 7.2 隐藏机器协议

隐藏子命令不出现在顶层 `--help`，但必须在 machine protocol 文档中版本化固定：

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json
slipway _machine submit --run RUN --action ACTION (--outcome-file FILE | --outcome-stdin)
slipway _machine answer --run RUN --action ACTION --text TEXT
slipway _machine answer --run RUN --action ACTION --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway _machine skip --run RUN --action ACTION
slipway _machine resume RUN [--budget N]
slipway _machine resume RUN (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway _machine material --run RUN --action ACTION --section KEY
```

所有操作接受并传播原 workspace `--root` 或等价 workspace identity。Ad-hoc Run 的 resume 不接受 source flags；Issue-bound resume 必须在 refresh (`--source-file`)、显式旧 snapshot (`--use-pinned-source`) 或处理当前 candidate (`--source-choice`) 三种互斥模式中选择，省略模式不得被解释为 source unchanged。

## 7.3 Resume budget 语义

`--budget N` 的 `N` 必须为 `1..1000`，表示成功 resume 后**替换**剩余 Action budget，不是累加：

- 显式提供 `--budget N`：在同一成功 mutation 中把 remaining budget 设为 `N`，随后 fresh Orient 消耗一个 Action；
- 省略时若仍有正余额：保留该余额；
- 省略时若余额已耗尽：设为 `max(initial_budget, 3)`，随后 fresh Orient；`3` 是让恢复至少能再次 Orient 并继续一小段的前进默认值，不是完成保证；
- source refresh 产生 candidate 时尚未真正 resume，因此不应用也不消耗该调用上的 budget replacement；响应必须明确报告 `budget_applied=false`，用户可在后续 current-candidate choice 上再次显式传 `--budget N`。不得静默记成已应用。

Budget 只限制 CLI 签发的 Action 数；用户可显式选择更小/更大的新预算，它不证明工作 ready、正确或完成，也不把失败变成 gate。

---

# 8. Change source envelope 与 pinned snapshot

Run 不能只保存可变的 `#42`，也不能信任宿主自行总结后的 Requirements。

## 8.1 Raw source envelope

宿主传给 CLI 的严格输入：

```json
{
  "source_version": 2,
  "provider": "github",
  "host": "github.com",
  "repository_id": "R_...",
  "issue_id": "I_...",
  "issue_number": 42,
  "canonical_url": "https://github.com/user/repo/issues/42",
  "updated_at": "...",
  "fetched_at": "...",
  "title": "[Change] ...",
  "body": "<!-- slipway-level: change/v2 -->\n\n<unique strict slipway-manifest JSON fence>",
  "labels": ["level:change", "kind:bug"],
  "parent": {
    "repository_id": "R_...",
    "issue_id": "I_...",
    "canonical_url": "https://github.com/user/repo/issues/40"
  },
  "comments": [
    {
      "node_id": "IC_...",
      "database_id": 101,
      "url": "https://github.com/user/repo/issues/42#issuecomment-101",
      "updated_at": "...",
      "author_id": "U_...",
      "is_minimized": false,
      "body": "<!-- slipway-section:v1 key=outcome -->\n..."
    }
  ]
}
```

`parent` 可省略，并只用于 traceability；父 Objective 的内容不构成运行时继承。对 valid manifest，`comments` 必须仅且完整包含 manifest 引用的 comments；CLI 不扫描或接收未引用的普通讨论 comments。无法解析 v2 manifest 的 refreshed head 使用已初始化的空 `comments` array，使 core 能分类 invalid candidate 而不收集无关讨论。

输入约束：

- unknown fields、重复 JSON key、错误类型、无效 UTF-8、BOM、控制字符和 trailing data 拒绝；
- source file 必须是一次性 no-follow 打开的普通文件，最大 16 MiB raw envelope；读取后立即关闭，不从同一路径二次读取；
- 只支持 `https://github.com/<owner>/<repo>/issues/<number>`，URL 不得含 userinfo、token、fragment 或非默认端口；
- `repository_id` 与 `issue_id` 是 GitHub node ID；number/title/URL 只是当前 projection；raw observation 最多 100 个 labels；
- 新 Run 的 body 第一个非空行必须是唯一 `change/v2` marker，下一非空 block 必须是唯一严格 `slipway-manifest` JSON fence；manifest 最大 256 KiB、有 5..64 个有序 section entries，并至少覆盖 `outcome|requirements|acceptance_examples|constraints|non_goals` 五类 role；
- 每个 manifest entry 把 stable key/role/title 绑定到 comment GraphQL node ID、REST database ID hint 与完整 comment body digest；被引用 comment 第一个非空行必须精确为 `<!-- slipway-section:v1 key=KEY -->`；
- Resume 的 envelope 先验证 identity/JSON，再分类 marker/manifest/section：非法、Objective、v1 或未知 marker 可以作为 drift evidence，但永远不能 adopt 为本 Run 的 Change source；
- labels 用于 drift warning，不参与 Level authority；
- Project fields、Issue Fields、attachments、linked pages 与未被 manifest 引用的 comments 不进入 envelope；
- 宿主是 trusted fetch attester。CLI 只能验证 envelope 内部一致性，不能证明其确实来自 GitHub。

## 8.2 确定性解析与 revision

CLI 自己把 Issue body manifest 解析为有序 chapter catalog，并只按 manifest identity 匹配 envelope comments。Manifest array 是 canonical chapter order 和唯一 source HEAD；comment 显示顺序、时间戳与 marker-like 普通讨论都不产生 authority。

每条被引用 comment 的第一个非空行必须精确匹配其 key 的 `<!-- slipway-section:v1 key=KEY -->`；其后 payload 只做 CRLF/CR → LF。除此之外不进行模型总结、列表重排、Unicode normalization 或语义改写，其余 UTF-8 bytes 原样保留。单个 normalized chapter 最大 256 KiB，完整 bundle payload 最大 4 MiB，manifest 最大 256 KiB，chapter 最多 64 个。缺失、额外、重复、minimized、被原地编辑、marker 错误或 hash 不匹配均 fail closed。

每章 payload 必须先以 `0600` 内容寻址 blob 写入 Run 私有 `materials/` 并 fsync，journal event 才能引用。版本化 digest 使用 domain-separated、length-prefixed framing；`frame` 中每个 UTF-8 field 前置一个 8-byte big-endian byte length：

```text
body_sha256 = SHA-256(frame("slipway-comment-body/v1",
                           exact normalized full comment body))

material_sha256 = SHA-256(frame("slipway-material/v1",
                               exact normalized chapter payload))

section_revision = SHA-256(frame("slipway-section/v2", key, role, title,
                                exact normalized chapter payload))

manifest_revision = SHA-256(frame("slipway-manifest/v2", manifest_version,
                                 profile, parent_requirements_revision_or_empty,
                                 ordered (key, role, title, comment_node_id,
                                          comment_database_id, body_sha256)))

requirements_revision = SHA-256(frame("slipway-requirements/v2", profile,
                                     ordered (key, role, title, section_revision)))

source_revision = SHA-256(frame("slipway-source/v2", source_version, host,
                               repository_id, issue_id, normalized title,
                               manifest_revision))
```

Timestamps、URLs、authors 与 fetch metadata 只是 provenance，不决定 requirements identity。REST database ID 也是 provenance hint，但 manifest revision 显式提交每个 referenced comment 的 GraphQL node ID 与 REST database ID，防止同一 manifest head 下的 immutable binding 漂移；GraphQL node ID 仍是 canonical remote object identity。Manifest array order 是 canonical chapter order；requirements revision 排除 provenance。

CLI 持久化的 pinned source 只保留 identity、有序 catalog、provenance、byte count 与 domain-separated revisions：

```json
{
  "source_version": 2,
  "parser_version": 2,
  "manifest_version": 2,
  "profile": "change/v2",
  "provider": "github",
  "host": "github.com",
  "repository_id": "R_...",
  "issue_id": "I_...",
  "issue_number": 42,
  "canonical_url": "...",
  "url_aliases": [],
  "source_revision": "sha256:...",
  "manifest_revision": "sha256:...",
  "requirements_revision": "sha256:...",
  "title": "...",
  "parent": {"repository_id": "...", "issue_id": "...", "canonical_url": "..."},
  "sections": [
    {
      "key": "requirements",
      "role": "requirements",
      "title": "Requirements",
      "body_sha256": "sha256:...",
      "section_revision": "sha256:...",
      "material_sha256": "sha256:...",
      "bytes": 18231,
      "provenance": {
        "comment_node_id": "IC_...",
        "comment_database_id": 102,
        "url": "https://github.com/user/repo/issues/42#issuecomment-102",
        "author_id": "U_...",
        "observed_updated_at": "..."
      }
    }
  ]
}
```

完整 raw body、comment marker 与 chapter Markdown 不进入 journal、status、candidate 或 Action；normalized payload 只存在 Run-private content-addressed material blob 中，catalog 通过 digest 引用它。Accepted Requirements 仍可能敏感，必须按 §12 的隐私边界处理。
`parser_version: 2` 只标识 strict v2 parser/profile；manifest mode 以 `manifest_revision` 标识 source HEAD，不再使用 v1 的 `parser_version + five section Markdown` requirements framing。

## 8.3 Amendment 与不可用来源

Issue-bound resume 必须选择且仅选择一种模式：

1. `--source-file FILE`：刷新并比较；
2. `--use-pinned-source`：用户在刷新失败或不想联网时明确继续旧 snapshot；
3. `--source-choice pinned|adopt --candidate CANDIDATE`：处理 journal 中 ID 完全匹配的 current candidate。

比较顺序固定，不允许提前 return 跳过后续校验：

1. 先验证 provider/host/JSON 与 issue node ID。provider/host 不同或 issue ID 不同，在任何 mutation 前拒绝并要求新 Run；
2. repository/number/URL 变化只标记 transfer/redirect projection，记录旧 canonical URL alias，并继续执行 marker/manifest/section 校验与 revision 比较，不能因 transfer 跳过 amendment；
3. 先校验当前 body 是否仍是 §4.4 定义的 manifest-addressed structurally valid Change：唯一受支持 `change/v2` marker、唯一严格 manifest、完整且仅包含被引用 comments、每章 marker/digest/size/identity 有效。CLI 不在此主观判断需求是否“足够好”或真正 self-contained；Objective、v1、未知/多个 marker、invalid manifest，或缺失、额外、重复、minimized、被原地编辑、marker/hash 不匹配的 referenced comment，生成 run-local unique `candidate_id`，写只含 identity/digests/classification error 的 invalid `source_candidate`，不保存 raw body，原子 void outstanding Action、queued suggestion 和 grant 并进入 `paused + decision_required`；
4. structurally valid Change 才计算 manifest/requirements/source revisions。manifest revision 未变化——包括 identical、projection-only 与其他 non-material drift——记录 refresh/projection drift，原子 void outstanding Action、queued suggestion 和 grant、恢复 active 并 fresh Orient；
5. 对于任何新的 manifest revision（包括 content-identical comment replacement），只有 refreshed manifest 声明的 `parent_requirements_revision` 与当前 pinned `requirements_revision` 完全相等时，才生成 run-local unique `candidate_id`、写 normalized valid `source_candidate`、原子 void outstanding Action、queued suggestion 和 grant、进入 `paused + decision_required`，且本次 resume 的 budget replacement 报告 `budget_applied=false`；若两者不相等，表示 amendment 基于不同 parent 编写，构成 history fork，必须在生成 candidate 或执行其他 mutation 前以 `source_history_fork` 拒绝并要求从 refreshed source 启动新 Run，绝不把该 fork 当作 candidate；
6. candidate pause 只接受 `slipway _machine resume RUN --source-choice pinned|adopt --candidate <current-id>`：ID 不匹配拒绝；`pinned` 保留 accepted manifest、Requirements 与 section content，同时应用 candidate 的 same-Issue repository/number/canonical-URL/alias/parent projection；`adopt` 只允许 valid same-issue candidate 并安装该 snapshot；invalid candidate 只允许 `pinned`；
7. `pinned` 后保留旧 decision context 并 fresh Orient；`adopt` 只有在 `requirements_revision` 改变时才将旧 Requirements 派生的 decisions 从 active context 移除，历史 answers 仅保留为叙事；content-identical manifest-only replacement 保持这些 answers active；
8. candidate 存在时再次传 `--source-file`/`--use-pinned-source` 拒绝；幂等键为 `(candidate_id, choice)`，同键重试返回原结果，不同 choice 或迟到 candidate ID 拒绝；
9. provider 暂时不可用时，宿主先报告失败；只有用户明确选择后才使用 `--use-pinned-source`。该操作写 `source_refresh_skipped`，原子 void outstanding Action、queued suggestion 和 grant 并 fresh Orient，不得把未刷新称为 unchanged；
10. `404`、失权、repository 变私有或 Issue 消失统一记为 `source_unavailable`；已有 Run 可显式使用 pinned snapshot，新 issue-bound Run 因无法取得 raw envelope 而不能启动；
11. accepted comment identity 跨 manifest heads 不可变。Replay 从每个 pinned manifest head 的 history 派生 accepted-comment identity ledger；retire 一个 comment 不会忘记其 node/database identity，之后 reintroduce 该 identity 必须匹配最初 accepted section；
12. accepted chapter 不原地编辑；amendment 创建 replacement comment，并在最后一步提交声明 `parent_requirements_revision` 的新 manifest。缺失、minimized、edited、duplicated 或 hash-mismatched referenced comment 均 fail closed；现有 Run 只能从用户显式选择的 local pinned bundle 继续；
13. 已交付需求后来变化：创建新 Change 并写 `Supersedes #N` 文本约定，不重写旧关闭 Issue 伪造历史。

任何成功的 issue-bound resume mutation 都必须在同一 journal transaction 中 void outstanding Action、清空 queued suggestion 并失效 destructive grant；任何后继只从 fresh Orient 产生。

一个 Run 最多一个 primary Change；一个 Change 可以有多个 Run。Snapshot/candidate 是 Run-local 执行输入，不是 Project Spec、Change runtime、freshness gate 或交付证明。

---

# 9. Action / Outcome 与 Run 状态协议

Machine protocol 使用 `contract_version: 2`；未知版本拒绝，不提供兼容解析器。

## 9.1 资源上限

```text
source-file / raw envelope      16 MiB
manifest                       256 KiB
single normalized section      256 KiB
complete bundle payload          4 MiB
outcome-file / outcome-stdin     1 MiB
single journal JSONL record      4 MiB
Action context                 128 KiB
suggested_actions count              1
suggested action brief           8 KiB
Action total encoded size          256 KiB
```

外部输入超过对应限制时在持久化前拒绝；所有文本为 UTF-8。`context` 是 CLI 生成的 bounded projection，不因历史自然增长而报错；若不可截断的 requirements catalog/reader 或 source 字段使完整 Action 仍超过总上限，则在签发前返回结构化 `action_too_large`，不得写半条 Action。

## 9.2 Action

```json
{
  "contract_version": 2,
  "run_id": "...",
  "action_id": "...",
  "kind": "orient",
  "goal": "...",
  "brief": "...",
  "context": "...",
  "source": {
    "kind": "change_issue",
    "canonical_url": "https://github.com/OWNER/REPOSITORY/issues/123",
    "issue_id": "I_...",
    "source_revision": "sha256:...",
    "manifest_revision": "sha256:...",
    "requirements_revision": "sha256:..."
  },
  "requirements": {
    "requirements_revision": "sha256:...",
    "sections": [
      {
        "key": "requirements",
        "role": "requirements",
        "title": "Requirements",
        "section_revision": "sha256:...",
        "material_sha256": "sha256:...",
        "bytes": 18231
      }
    ],
    "required_for_action": ["requirements"],
    "reader": {
      "operation": "read_material",
      "base_argv": [
        "slipway", "_machine", "material", "--root", "/absolute/workspace",
        "--run", "RUN", "--action", "ACTION"
      ],
      "input": {
        "name": "section",
        "type": "enum",
        "flag": "--section",
        "required": true,
        "choices": ["requirements"]
      }
    }
  },
  "remaining_budget": 7
}
```

`kind` 只允许 `orient|clarify|implement|review|summarize`。每个 issue-bound Action 带 source/manifest/requirements revisions、有序 section catalog 和完整 structured `_machine material` reader，但绝不复制 Markdown；Ad-hoc Action 必须同时省略 `source` 与 `requirements`，不得发送 `null`。Host 用 reader 的 exact structured argv 解析所需 key，并收到一个经 digest、byte count 与 section revision 验证的 `action_material` JSON；该操作只读 Run-local blob，不访问 GitHub。只有 current non-void Action 可读，completed、replaced、stopped 或其他 stale Action 均拒绝。

`destructive_authorization` 通过 machine schema 正式接入 Action discriminated union：只有已经按 §11 确认 scope 的 `kind=implement` 分支要求且允许该字段，其值必须逐字段匹配 §11 的 canonical authorization object；所有其他 Action/Implement 分支都禁止该字段，不能发送 `null`。CLI 必须验证 `request_id`、`originating_action_id`、scope version/digest、targets、impact 与 current one-shot grant 完全一致。

`context` 只保存 active confirmed decisions 和前序 Outcome 的有界投影，不是 Requirements authority、raw Issue、comments、journal 全量或 agent hidden reasoning。组装算法固定：

1. 候选项按优先级为 current/latest confirmed decision、其他 active decisions（新到旧）、最近 Outcome summary/known issues、其余 Outcomes（新到旧）；被后续答案推翻的 decision 不进入候选；
2. 每个候选项先做 UTF-8 校验和 LF 规范化；单项超过剩余空间时在 UTF-8 code-point 边界截断，并附 `...[truncated original_bytes=N sha256=...]`；
3. 按优先级选择到 128 KiB，最终按每一类的时间顺序渲染；被省略时写明每类 omitted count，不能声称“完整历史”；
4. 相同 journal 必须得到 byte-identical context。CLI 不调用模型做二次总结，normative materials 不与 context 争夺额度，已省略历史仍只作为 journal narrative 保留。

## 9.3 Outcome

```json
{
  "contract_version": 2,
  "action_id": "...",
  "action_kind": "orient",
  "status": "completed",
  "summary": "observed facts",
  "observations": [],
  "known_issues": [],
  "suggested_actions": [],
  "pause": null,
  "implementation": null,
  "review": null
}
```

`suggested_actions` item：

```json
{"kind": "clarify", "brief": "one bounded next decision"}
```

`pause`：

```json
{
  "reason": "decision_required",
  "question": "one human decision",
  "destructive_request": null,
  "supersedes_answer_action_id": "previous-answer-action-id"
}
```

破坏性请求：

```json
{
  "reason": "destructive_confirmation_required",
  "question": "Confirm this exact destructive scope?",
  "destructive_request": {
    "request_id": "...",
    "targets": [{"kind": "path", "value": "/absolute/target"}],
    "impact": "exact irreversible consequence",
    "scope_sha256": "sha256:..."
  }
}
```

Implement result：

```json
{
  "result": "applied",
  "files_changed": [],
  "activities": [
    {"kind": "test", "command": "exact command", "exit_code": 0, "summary": "..."}
  ],
  "uncertainties": [],
  "attempts": 1
}
```

`result` 只允许 `applied|partial|not_needed|unable`；activity kind 只允许 `test|typecheck|build|lint`。零 activity 合法，最终报告必须固定写：

```text
No test, typecheck, build, or lint activity was reported.
```

Review result：

```json
{
  "result": "findings_reported",
  "findings": [
    {"location": "path:line or surface", "summary": "...", "detail": "..."}
  ],
  "uncertainties": []
}
```

`result` 只允许 `no_findings_reported|findings_reported|inconclusive|not_run|error`。

Host Outcome 的 `status` 只允许 `completed|needs_input|partial|error`；`skipped` 只由 CLI 的 `slipway _machine skip` 事件生成，宿主不能提交。所有公共 Outcome 字段必须出现；其中 `action_kind` 必填，且必须与当前已签发 Action 的 `kind` 完全相同；缺失、未知或不匹配一律拒绝，不从 Action ID、status 或 result branch 推断。非本 Action 的 `pause|implementation|review` 使用 JSON `null`，不能省略。

## 9.4 合法组合矩阵

| Action | Host status | Required result combination | Allowed pause | Allowed suggestions |
| --- | --- | --- | --- | --- |
| Orient | completed/partial/error | implementation=null, review=null | none | Clarify、Implement 或 Summarize，0..1 |
| Orient | needs_input | implementation=null, review=null | decision/environment | none |
| Clarify | completed/error | implementation=null, review=null | none | Clarify、Implement 或 Summarize，0..1 |
| Clarify | needs_input | implementation=null, review=null | decision/environment | none |
| Implement | completed | implementation.result=`applied\|not_needed`, review=null | none | none |
| Implement | partial | implementation.result=`partial`, review=null | none | none |
| Implement | error | implementation.result=`unable`, review=null | none | none |
| Implement | needs_input | implementation=null, review=null | decision/destructive/environment | none |
| Review | completed | review.result=`no_findings_reported\|findings_reported`, implementation=null | none | none |
| Review | partial | review.result=`inconclusive`, implementation=null | none | none |
| Review | error | review.result=`error`, implementation=null | none | none |
| Summarize | completed/error | implementation=null, review=null | none | none |

规则：

- `needs_input` 必须有 `pause`；其他 status 的 `pause` 必须为 null；
- Clarify 的 `partial` 组合有意不合法：一个 Clarify Action 只承载一个决定；未得到决定时返回 `needs_input`，已完成则 `completed`，无法继续则 `error`。不得用 partial 暗藏第二个问题或半个决定；
- Orient/Clarify 的非 `needs_input` Outcome 若 `suggested_actions=[]`，CLI 确定性路由到 Summary；不得留下没有 outstanding Action 的 active Run；
- pause reason 只允许 `decision_required|destructive_confirmation_required|environment_unavailable`；`budget_exhausted` 只由 CLI 生成；
- destructive request 只允许 Implement + destructive pause；
- Review 不得 `needs_input`、不得建议 Implement；`not_run` 只由 CLI review-skip projection 生成；
- Summary 不得建议后续动作；
- incompatible result/status、未知字段、错误版本、重复 JSON key，以及缺失、未知或与当前 Action `kind` 不一致的 `action_kind` 全部拒绝；
- verdict、approved、gate、freshness、done-ready 与 ship-ready 不是协议字段，严格 schema 会拒绝；
- `docs/reference/machine-protocol.schema.json` 必须用 discriminated `oneOf` 精确实现本矩阵，所有文档示例、Go types、模板和 golden tests 都由该 schema 验证。

## 9.5 Run 状态

```text
active
paused
stopped
ended
```

- 新 Run 创建为 `active`；普通 `decision_required` answer、non-material source refresh、source choice 或 resume 成功后为 `active` 并立即签发 fresh Orient；destructive grant answer 是唯一直接签发 scoped Implement 的例外；
- valid/invalid source candidate 原子 void 旧 Action/grant并进入 `paused + decision_required`，不会留下仍可提交的旧 Action；
- 其他 decision、destructive、environment 或 budget pause 为 `paused`；
- `slipway stop` 进入 `stopped`，不删除 journal；
- Summary Outcome 被接受后进入 `ended`；Summary 被 skip 时写 CLI minimal summary event 后进入 `ended`；
- `paused`、`stopped` 和宿主消失但仍有 outstanding Action 的 `active` Run 可以 resume；
- `ended` 不可 resume，但同一 Change 可启动新 Run；
- fresh Orient 在普通 decision answer、source comparison 无 material candidate、candidate 已选择或显式 pinned-source/ad-hoc resume 后签发；environment pause 不接受 answer，只能在环境恢复后 resume。

## 9.6 幂等、迟到提交与结构化 answer

- 一个 `action_id` 只接受一个 Outcome；完全相同 payload digest 的重复提交幂等返回已记录结果和当前 next Action；不同 payload 返回 conflict；
- 非当前、已 void、Resume 前、stopped 或 ended Run 的迟到提交拒绝；
- contract version mismatch 拒绝，并建议 `slipway install --refresh`；
- `answer` 只接受当前 paused Action；同一结构化 answer digest 重试幂等，不同 answer conflict；
- 普通 `decision_required` 的 `answer --text` 记录决定、void当前 Action/queue并签发 fresh Orient；`environment_unavailable` 拒绝 answer，只能 resume；source candidate 没有 host Action，只接受 resume choice；
- `pause.supersedes_answer_action_id` 是可省略字段，且只允许用于 `reason=decision_required`；其值必须是一个已记录、仍为 active、不属于 `--confirm-destructive` authorization attestation 且属于当前 Requirements revision（ad-hoc Run 则属于当前 ad-hoc Requirements context）的 answer 的 `action_id`。该字段记录“下一份回答将取代哪一份旧决定”的显式意图；只有用户向这个 waiting Action 提交新的 decision answer 后，CLI 才把被点名的旧 answer 标为 inactive（仍保留在历史中）并记录新 answer 为 active。Skip 不使旧 answer 失效，也不得从 prose 推断或连带停用其他 answers；
- source amendment choice 只能使用 `slipway _machine resume RUN --source-choice pinned|adopt --candidate CANDIDATE`，不得从自然语言推断；candidate ID 必须匹配 current journal candidate，`adopt` 仅允许 same-issue、manifest-addressed structurally valid candidate；
- destructive grant 只能使用 `--confirm-destructive --scope-sha256`，不得从 `--text` 推断；成功后签发唯一一条携带 grant 的 scoped Implement；
- `skip` 永远不要求理由，并生成 CLI-owned skipped event；
- skip 转移固定：Orient/Clarify/Implement skip 后先应用 observed-diff-first 路由（CLI 观察到 start fingerprint 以来的 diff 且 review enabled → Review，否则 Summary）；Review skip → Summary；Summary skip → CLI minimal summary + ended；
- 每次成功 mutation 必须返回当前状态和结构化 `next`。

---

# 10. 软自动驾驶算法

默认路径：

```text
orient
  ├─ 事实可自行查明 → 继续调查
  ├─ 存在人的决定 → clarify，一次一个
  └─ 目标已明确 → implement
                         │
                         ├─ CLI observed any diff since immutable Run-start fingerprint + review enabled → review
                         │    （即使 host 声称 not_needed/unable，也只记录 observation/report discrepancy 与 attribution uncertainty）
                         └─ no observed-since-start diff 或 --no-review → summarize

review（任何合法结果） → summarize
summarize（任何合法结果） → ended
```

规则：

- Orient/Clarify 最多建议一个 immediate next Action；每个 Action 可无理由 skip；
- dependent decision 必须作为下一条 Clarify suggestion 或 pause question 表达，不能只写 prose；
- failed/missing activity、Review finding、dirty worktree 与 Issue 状态不控制 progression；
- Review finding 进入 Summary，不自动创建修复、新 Change 或 re-review；
- Implement 的内部修复尝试上限写入 brief，Outcome 如实报告实际次数；CLI budget 只限制 Action 数，不谎称约束单个宿主调用内部循环；
- budget 用尽由 CLI 进入 `budget_exhausted` pause；resume 补充预算并重新 Orient；
- Run start 固定 canonical workspace identity、HEAD、index tree/fingerprint、`git status --porcelain=v2 -z --untracked-files=all` 结果，以及每个 pre-existing dirty/untracked path 的内容 digest 或明确的 unreadable/oversize observation；后续 Git observation 不修改 start snapshot；
- CLI 只能通过完整 start fingerprint 判断是否存在 start-to-current diff，不只信任 `files_changed`；一旦存在，记录正式 machine observation `observed_since_start`。该 diff 是安全侧路由信号而非因果归因：并发用户编辑、其他 Run 或工具都可能贡献差异。任何该差异在 review enabled 时优先触发 Review；`not_needed|unable` 与差异并存，或 `applied|partial` 却无差异时，记录 observation/report discrepancy 和 attribution uncertainty，不指控 host claim mismatch，也不声称“本次 Run 改动”；
- 同一 Change 的新 finding 由用户决定形成新 Change 或新 Run；当前 Run 不自动扩张。

---

# 11. 破坏性确认与 trusted-host 边界

`--confirm-destructive` 是**可信宿主对用户当前确认的结构化 attestation**和防止自然语言误解释的机制，不是密码学不可伪造的人类存在证明。拥有 shell 权限的恶意宿主技术上可以伪造 CLI flag；文档、技能和安全模型必须如实说明这一边界。

Destructive target 必须是非空 typed list，kind 只允许 `path|git_ref|external_resource|data_domain`，value 为非空 UTF-8；去重后按 `(kind,value)` bytewise 排序。CLI 对以下对象使用 RFC 8785 JSON Canonicalization Scheme，再计算 SHA-256：

```json
{
  "scope_version": 1,
  "request_id": "uuid",
  "targets": [{"kind": "path", "value": "/absolute/target"}],
  "impact": "exact irreversible consequence"
}
```

`impact` 不得为空；CLI 必须自己重算 digest，不能信任宿主自报。

流程：

1. Implement 返回 `needs_input + destructive_confirmation_required + destructive_request`；
2. request 使用 UUID `request_id`、至少一个 typed target、非空 irreversible impact 与 CLI 可重算的 canonical `scope_sha256`；
3. CLI 暂停，不签发任何 destructive grant；
4. 用户在宿主的人类交互面明确确认后，宿主调用：

   ```bash
   slipway _machine answer --run RUN --action ACTION \
     --confirm-destructive --scope-sha256 sha256:...
   ```

5. CLI 重算并验证 digest 与当前 request 完全一致，写入一次性 grant；
6. 下一条 Implement Action 必须逐字复制 canonical request scope：

   ```json
   {
     "request_id": "uuid",
     "originating_action_id": "...",
     "scope_version": 1,
     "scope_sha256": "sha256:...",
     "targets": [{"kind": "path", "value": "/absolute/target"}],
     "impact": "exact irreversible consequence",
     "confirmed_at": "..."
   }
   ```

7. 空 targets、重复/未知 kind、digest mismatch 或缺失 impact 必须拒绝；
8. grant 只对该下一 Action 和精确 scope 有效；Action 完成、skip、stop、resume、source amendment、target/impact 改变后立即失效；
9. 执行者发现实际 scope 超出 grant 时必须返回新的 destructive request，不能部分解释旧确认。

任意自然语言文本，包括 “yes”，在没有结构化 flag 时都不确认。普通 answer 记录为拒绝/反馈并重新 Orient 非破坏性替代方案；不得重新签发带破坏权限的 Implement。

---

# 12. Run journal、恢复与隐私

Run journal 存在 Git common directory 下，每个 linked worktree 记录自己的 canonical workspace identity，但 Slipway 不创建、切换、绑定或删除 worktree。

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl
├── run.json
└── run.lock
```

## 12.1 事件与 projection

- append-only delta events 是权威；`run.json` 只是可重建 projection；
- 记录原始 goal、workspace identity、完整 start Git fingerprint、pinned source、Actions/Outcomes、answers、skip/stop、source choice、destructive request/grant、budget 与 known issues；
- 不记录 raw Issue body、完整 comments、Project fields、rendered shell command 或 hidden chain-of-thought；
- single journal record 最大 4 MiB，逐记录 streaming replay；interrupted final tail 可以安全修复；
- 已存在 journal 的 mutation 只有 event bytes 写完且 journal handle `fsync` 成功才 committed；首次创建 run/journal 还必须 fsync 新文件、run directory 和创建目录项的 parent directory；
- projection 通过 temp write + file fsync + atomic rename + parent-directory fsync 刷新。Journal commit 后 projection 任一步失败必须返回“mutation committed, projection stale”，不能诱导盲重试；
- run 创建、event append、projection rename 和 directory sync 都有 crash/fault-injection 测试；不支持目录同步的平台必须收窄 durability 承诺并在 doctor/documentation 明示；
- 重建 projection 必须确定性地产生同一状态和 next operation。

## 12.2 Root anchoring 与同 UID 并发威胁

威胁模型包含同一用户权限下另一个进程并发替换 run directory、journal、lock、symlink、junction 或 reparse point。

必须同时明示跨平台删除边界：Darwin/Linux/POSIX 的 `unlinkat`/目标平台等价调用只按“已锚定 parent handle + leaf pathname”删除，不提供 portable compare-and-unlink，也不能从已打开 leaf handle 线性化删除某个已验证 directory entry。因此，Slipway 不承诺抵御一个持续主动的同 UID watcher 在**最后一次 leaf identity validation 返回之后、pathname unlink/rmdir 系统调用取得对象之前**替换该 entry。实现仍必须使用长期 identity pin、私有随机 quarantine、atomic no-replace relocation、relocation 后 revalidation 与 post-check 缩小窗口；在任何可观察 validation point 发现 mismatch 时必须保留并报告。root、malware、同账户持续竞速该最终 syscall gap 属明确 residual limitation，不能被 C test 或“随机名字”伪装成已消除；某个平台只有实际使用 exact-object native deletion 时才可声明更强保证。

在这个明确边界内：

- 使用 anchored `os.Root`/native directory and file handles，不以“先验证路径、后按路径重开”代替 anchoring；
- 打开后保存 run directory 与 leaf file identity，在 append/fsync 前后验证 identity 与 namespace 仍一致；
- interrupted-tail 检测与 truncate 在同一已验证 file handle 上完成；
- 可替换的目录内 lock 本身不作为 namespace identity；
- parent/run directory 被交换时安全失败并报告已提交/未提交的确定状态；
- Windows 以 handle/reparse-point 语义提供等价保护；任何需要重建 symbolic link、却无法在当前权限下证明可精确恢复的 transaction，必须在第一次 mutation 前 fail closed。安全拒绝是契约内结果，不能先移动对象、再依赖 symlink privilege 回滚，也不能以 privilege-dependent Skip 作为唯一证据；
- root/leaf swap、lock replacement、tail repair race 和 rollback concurrent-edit 必须有对抗测试。

## 12.3 恢复、并发与工作区

- per-run lock 只保护 journal 完整性，不约束 Git、Issue 或用户工作；
- 同一仓库允许多个 Run 并存；歧义时 stop/resume 要求显式 Run ID；
- 同一 Change 的并发 Run 只提示风险，不建立 hard lock；
- recovery 必须重新验证 canonical workspace identity；从 linked worktree B 恢复 A 时仍只能观察/修改 A，除非用户显式启动新 Run；
- 删除 run directory 只删除恢复能力，不改变仓库、Issue 或交付状态。

## 12.4 隐私边界

不能绝对承诺“journal 不含 secret”，因为 goal、accepted Requirements、answers 和 command summaries 都来自用户/宿主内容。真实承诺是：

- 不主动收集 token、credential、完整 comments、环境变量或与任务无关的文件内容；
- source import 只持久化 accepted sections，不持久化 raw body；
- generated capability 在写 journal 前提醒用户不要输入 secret，并最小化/显式 redact 已识别凭据；
- journal 使用当前用户最小文件权限；Windows 使用用户 ACL 等价保护；
- 文档明确保留期、备份风险和删除 run directory 的 purge 方式；
- 删除只影响恢复，不被描述成安全擦除或密钥销毁。

## 12.5 Legacy namespace 残留

新 runtime 在 `<git-common-dir>/slipway/` 下只认领本节定义的 `runs/<run-id>/` 严格布局；旧版本留下的 `runtime/`、`cache/`、`scope-root`、`scopes/`、`locks/`、`processes/`、`repair-backups/` 以及其他未知 sibling 一律视为**未归属的 legacy residue**：

- run/list/status/replay 必须忽略，不迁移、不导入、不让其改变当前 Run 状态；
- `doctor` 可以用稳定 advisory code 枚举检测到的 legacy top-level path，并链接手工备份/清理说明，但不得读取其中任务内容、把 residue 判成不健康、返回阻塞性 exit code 或要求清理后才能 Run；
- install/refresh/uninstall 不删除、不改名、不接管这些路径；只有 manifest 与当前版本精确 ownership 规则证明拥有的文件才可删除；
- 文档可建议先停止旧二进制、备份后由用户手工清理，并明确这不是自动迁移、兼容 runtime、freshness check 或交付 gate。

这一定义允许旧数据与新 `runs/` 共存，但不承诺旧二进制能理解新 journal；同时运行不受支持的旧/新二进制只产生 advisory 风险提示，不授权 Slipway 杀进程或清理用户数据。

---

# 13. 安装、宿主与 ownership

支持十个宿主：

```text
Claude Code
Codex
GitHub Copilot
Cursor
Kilo Code
Kiro
OpenCode
Pi
Qwen Code
Windsurf
```

要求：

- 所有 public/hidden machine JSON 遵守 §7 的版本化稳定契约；`install --refresh` 是刷新入口；
- 只生成六个显式能力，无 ambient hook；
- doctor 对 GitHub 工作流只报告 `gh` 版本/认证/权限的 capability warning；GitHub 不可用不使 ad-hoc Run 整体 unhealthy；
- ownership manifest 只接受当前唯一版本和 host-specific 的精确生成路径；任何非当前版本、malformed、duplicate、out-of-host 或 unknown claim 在其 file claims 被用于任何 mutation 前 fail closed；
- 不读取 v1/旧 manifest 作为删除或替换证明，不维护旧路径 inventory，不迁移旧 manifest/marker/settings，不从旧格式推断 ownership；旧 manifest、marker-only state、retired hooks/settings 与其他未被当前 manifest 认领的内容保持未归属、原样保留，只允许用户显式手工处理；
- 任意伪造或非当前 manifest 不得授权删除 host root 内用户文件；
- refresh/uninstall 只依据当前版本 manifest 删除 hash-matching pristine managed files，保留并报告用户修改；
- install/uninstall report 必须区分 `transaction_outcome=committed|rolled_back|not_committed|ambiguous`；只有 committed 可保留 planned written/removed，普通 ownership preservation 放在 `preserved`，实际 recovery/quarantine artifact 单列 `recovery_artifacts`；
- `.adapter-generated` sentinel 只是 health evidence；missing 可 refresh 重建，modified 视为用户内容并由 refresh/uninstall 保留，doctor 只能建议检查并按需手工删除后重建；
- Copilot 自动探测接受 `.github/copilot`、`.github/prompts`、`.github/skills` 任一现有 host surface，生成能力写入 `.github/skills`；
- 全部文件 mutation 使用 root-anchored transaction、preconditions 和 rollback post-state validation；
- rollback 与 cleanup 在每个 identity validation point 保留并报告已观察到的并发用户修改；最终 pathname deletion gap 服从 §12.2 的 residual limitation，不宣称线性化 exact-object delete；
- Windows 不依赖 POSIX shell 或 Unix mode；对无法在当前权限下精确重建的 pre-existing symbolic link，transaction 必须在 mutation 前返回 typed fail-closed error，因而不依赖 symlink privilege 完成 rollback；
- 所有宿主模板共同包含 Issue untrusted-data、trusted fetch attestation、external publication reconciliation 和 destructive grant 边界，不能由各 adapter 自行发明。

---

# 14. 结构化恢复与跨平台 rendering

Machine-readable error/pause/status 不以 shell string 作为权威。需要用户输入或有多个安全选择时，返回可确定展开的 typed variants：
`next.operation` 只允许 `action|answer|resume|start|command|none`；除 `none` 必须使用空 variants 外，其余 operation 至少提供一个可执行 variant。

```json
{
  "next": {
    "operation": "resume",
    "workspace_identity": "...",
    "variants": [
      {
        "id": "refresh-source",
        "base_argv": ["slipway", "_machine", "resume", "RUN", "--root", "ABSOLUTE_ROOT"],
        "inputs": [
          {"name": "source_file", "type": "path", "flag": "--source-file", "required": true}
        ]
      },
      {
        "id": "use-pinned-source",
        "base_argv": ["slipway", "_machine", "resume", "RUN", "--root", "ABSOLUTE_ROOT", "--use-pinned-source"],
        "inputs": []
      }
    ]
  }
}
```

Ad-hoc Run 的 resume 返回一个无输入 variant：

```json
{"id":"resume-ad-hoc","base_argv":["slipway","_machine","resume","RUN","--root","ABSOLUTE_ROOT"],"inputs":[]}
```

该 variant 只对 journal 中无 source 的 Run 合法；issue-bound Run 使用上面的 source variants。

若 refreshed source 产生 material difference，CLI 已将 normalized candidate 持久化并暂停，因此返回无输入的 variants：

```text
keep-pinned: base_argv += ["--source-choice", "pinned", "--candidate", CURRENT_CANDIDATE_ID]
adopt:       base_argv += ["--source-choice", "adopt",  "--candidate", CURRENT_CANDIDATE_ID]
```

若 candidate structurally invalid，只返回 `keep-pinned`。Candidate 选择不依赖 ephemeral source file；next/status 必须显示 current candidate ID，迟到 ID 拒绝，相同 `(candidate_id, choice)` 可幂等重试。

确定性展开规则：

```text
resolved_inputs = []
for each input in schema order:
    append input.flag to resolved_inputs
    append the exact unquoted input value as one argv element to resolved_inputs
if base_argv contains the positional `--` separator:
    insert resolved_inputs immediately before that separator
else:
    append resolved_inputs to base_argv
```

`start` variant 使用固定的 `slipway run --budget N --json --root ROOT [--no-review] -- GOAL` grammar；因此包括 `-` 开头在内的 goal 始终是 `--` 后唯一的 literal positional value，typed input 不会落入 goal 区域。

规则：

- variant `id`、`base_argv`、input type/flag/order 与 workspace identity 共同构成机器权威；不得返回字面 `<answer>`/`<file>`；
- input type 只允许 `string|path|enum|digest`；enum 必须内列 choices，digest 必须验证算法前缀/长度；
- 所有必填 input 获得 typed value 后才形成 `resolved_argv`；无 input variant 的 `base_argv` 已可直接执行；
- 每个 hidden operation 的 next schema 都必须能无歧义展开到 §7.2 的完整命令，包括 `--source-file`、`--use-pinned-source`、`--text`、`--source-choice --candidate` 和 destructive flags；
- 所有 variant 包含原始 absolute root 与 workspace identity；
- 只有 resolved argv 完整时才生成 display command；human text output 按当前平台即时 rendering；
- rendered command 不写入 journal，projection/status 根据结构化 next 实时重建；
- source-file 导入后是 ephemeral，后续 recovery 不引用旧临时路径；Issue-bound resume 要求新 source 或显式 pinned variant；
- POSIX 使用 POSIX quoting；Windows 分别支持 `cmd.exe` 与 PowerShell；
- `%`/`!` 等 cmd expansion 风险使用 PowerShell UTF-16LE `EncodedCommand` 或等价安全 argv 路径；
- native Windows 测试实际通过 `cmd.exe /v:on` 和 PowerShell 捕获 resolved argv；
- 路径/参数包含空格、引号、Unicode、CR/LF、`%`、`!`、`&`、`^` 时仍保持原 argv；
- Issue URL、source-file、outcome-file、answer 和 root 同样覆盖；
- shell renderer 只负责展示/复制执行，不能改变 machine operation 语义。

---

# 15. 明确排除的旧表面

不提供或包装重现：

```text
旧 change/profile/lifecycle commands
model.Change / change.yaml authority
artifacts/changes/** runtime
S0/S1/S2/S3 state machine
artifact schema / requirements.md / tasks.md bundle
verification YAML / evidence digest / freshness
scope contracts / compiled done gates
worktree binding / worktree-preflight
SessionStart/UserPromptSubmit hooks
global router
install profile dependency closure
old config/state readers
compatibility aliases / state migration / dual mode
```

历史 artifact 与用户 worktree 不触碰、不迁移、运行时忽略。

新语义中的普通英文 “change” 只表示一个 GitHub Change Issue；它没有 CLI command、repository bundle、state machine 或完成认证。

---

# 16. 目标代码结构与依赖方向

```text
cmd/                    七个公开命令、隐藏机器协议和 text/JSON rendering
internal/autopilot/     Action/Outcome、Run、raw source validation、pinned snapshot、routing
internal/runstore/      anchored append-only recovery journal
internal/adapter/       十宿主 registry 与 ownership-safe transaction planning
internal/tmpl/          六个显式能力模板
internal/fsutil/        atomic/rooted transaction、Git discovery、symlink/reparse defense
internal/recoverycmd/   纯 POSIX/cmd/PowerShell display rendering；只消费结构化 argv
acceptance/             构建后二进制 E2E Shell、GitHub fixture 与宿主验收
```

依赖方向固定为：

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil（仅所需低层原语）
```

`autopilot` 只产生结构化 `next`，不得依赖 `recoverycmd`；`recoverycmd` 不读取 journal、不决定路由，只把完整 argv rendering 为人类 display command。不得用 wrapper package 绕过架构守卫。

不得引入：

```text
internal/change
internal/spec
internal/plan
internal/lifecycle
internal/gate
internal/tracker runtime
```

GitHub 读取/写入由宿主能力和用户已有认证工具完成；Go binary 只验证 raw envelope、确定性解析并持久化 pinned snapshot。Publication reconciliation 属于宿主外部操作纪律，不恢复 repository Change runtime。

# 17. 文档与发布

必须同步：

- README（英/中/日）；
- start-here、architecture、commands；
- 完整 machine protocol JSON Schema、合法组合矩阵和 hidden command reference；
- runs/source/privacy/trusted-host threat model；
- adapters、ownership 与 external publication reconciliation；
- issue workflow（Objective/Change、自包含规则、labels/body markers、GitHub limits）；
- Windows cmd/PowerShell rendering；
- acceptance requirements matrix；
- website 三语言内容与导航；
- Docker、Nix、GoReleaser、Homebrew/Scoop/AUR/包发布表面。

不得把 GitHub Project、PRODUCT.md、总体设计 artifact、Organization-only features 或 per-Issue privacy 写成前置条件。英文若只提供摘要，必须明确标为 non-normative；完整中文契约与 machine-readable schemas 才是实施权威，避免两个不完全镜像的“normative”文本产生冲突。

---

# 18. 验收场景与证据映射

证据类型：

```text
C  deterministic Go contract/property/race test：只证明 CLI/core 可观察语义或静态模板约束
S  acceptance 下实际调用构建后二进制的 Shell test
G  隔离 GitHub.com User-owned fixture 或 host-side fault-injection API harness
H  Claude/Codex/Pi 真实 prompt transcript + evaluator notes
W  native Windows cmd.exe + PowerShell test
R  docs/package/release validation
```

标签表示最适合该执行边界的证据类型，不是自动 pass/fail 或 progression gate。相关场景应在 `acceptance/README.md` 链接可重复取得的证据；缺口如实记录为 uncertainty。CLI/core 行为用 C/S，提示与宿主自主行为用 H，GitHub 原生语义用 G，Windows 声明用 W。组合标签表示互补视角；静态 C 不能替代模型行为 H，fake endpoint 也不能替代真实 GitHub fixture。本矩阵不参与 Run 路由或状态判断。

1. **显式启动 `[H]`**：普通聊天不启动 Slipway；用户明确运行后才开始 Orient。
2. **Stateless clarify `[C+H]`**：事实自行调查、决定交给人类、一次一个并给推荐；完整请求零问题；若 grilling 改变执行理解，用户确认共同理解后才 Implement；wrap-up 立即停止，不写文件、不创建 Issue、不偷渡 docs。
3. **Propose Change `[G+H]`**：展示自包含 requirements-only 草稿、approved plan 和全部外部写入；现有无 marker Issue 显示手工正文、新建链接 Change、ad-hoc Run 三选项；确认后 level marker、labels、operation/item markers 正确。
4. **Propose Objective `[G+H]`**：大型 feature/bug/refactor 创建 Objective，不自动 Implement。
5. **Decompose `[G+H]`**：Objective 拆为 self-contained vertical Changes，适用共同约束已物化，native sub-issues/blockers 正确，用户批准前不外写；显式 amendment mode 展示逐项 diff，不后台继承、不改关闭子项。
6. **Non-Organization repo `[G]`**：在 User-owned repository 中成功，不需要 Issue Types、Issue Fields、Organization 或 GitHub Project。
7. **Publication reconciliation `[G+H]`**：在 Claude/Codex/Pi 三个宿主以同一 fault script 模拟 timeout-after-success、部分关系失败、并发重复和索引延迟；不盲重试，返回 `created|matched|failed|ambiguous`，多个匹配暂停；另以隔离 live fixture 回读确认。
8. **Marker/projection drift `[C+G]`**：body marker 决定 Level；缺失/冲突 marker 拒绝；缺失 label 只警告且可经确认修复；`ready-for-agent` 不 gate Run。
9. **Deterministic source `[C+S]`**：同一 raw envelope 在所有平台得到相同 revisions/sections；任意 accepted content tamper 必须改变 CLI 重算 digest；Objective marker、重复 key、oversize、bad UTF-8、symlink/reparse source file 被拒绝。
10. **Untrusted Issue content `[C+H]`**：Issue 中的 prompt injection、无关 shell 指令、credential request 和恶意链接没有宿主指令权限。
11. **Issue-bound Run `[C+S]`**：identity、source/requirements revisions 与 accepted sections 在 status/journal/replay 中一致，raw body 不持久化。
12. **Amendment `[C+S+G]`**：requirements 改变时原子 void旧 Action/queue/grant并暂停；pinned/adopt 绑定 current candidate ID、迟到 ID 拒绝且同键重试幂等；transfer 后仍继续 marker/revision 比较。
13. **Unrefreshed/unavailable/transfer `[C+G]`**：缺 source 不能冒充 unchanged；显式 pinned 可继续；404/失权/消失统一 source_unavailable；transfer alias 重读且仍执行 marker/revision 比较。
14. **Ad-hoc escape hatch `[S+H]`**：GitHub 不可用、Issue 未规范化或用户不想建 Issue 时仍可显式启动、停止和无 source flags 恢复 bounded goal。
15. **Dependent decisions `[C+H]`**：一次一个；下一决定用 Clarify suggestion/pause 表达；普通 answer 必定 fresh Orient，environment 只能 resume，零 suggestion 必定 Summary。
16. **Submit/answer/candidate idempotency `[C+S]`**：相同 payload 或 `(candidate_id,choice)` 重试返回同结果；不同 payload/choice conflict；Resume 后迟到 Action/candidate 拒绝。
17. **Skip/stop/resume `[C+S]`**：无理由可跳过，stop 可恢复；任意成功 resume 原子清空 outstanding Action/queue/grant并 fresh Orient，ended 不可 resume。
18. **Failed activity `[C+S]`**：真实非零 exit code 被记录并继续 Summary，不作为 gate。
19. **Review findings `[C+S+H]`**：只报告，不编辑、不 `needs_input`、不建议 Implement、不暂停、不 re-review。
20. **Destructive scope `[C+S+H]`**：自然语言 yes/no 不确认；digest mismatch、扩围、Resume 和 Action 变化使 grant 失效；只有精确结构化 grant 进入下一 Implement。
21. **Bug/refactor granularity `[G+H]`**：小工作为 Change，大工作为 Objective + self-contained Changes；Level 与 Kind 正交。
22. **No activity truth `[C+S]`**：未启动的活动不生成虚假 activity；零活动使用固定表述。
23. **Current-worktree observation `[C+S]`**：固定 start HEAD、index/status、pre-existing dirty/untracked content digests 与 workspace identity；后续 observation 不污染 start snapshot；并发用户/其他 Run 修改时仍触发安全侧 Review，但报告 `observed_since_start` 与 attribution uncertainty，不声称 Run-attributable 或 host claim mismatch。
24. **Runstore namespace attacks `[C+W]`**：parent/run/leaf swap、lock replacement、tail-repair race、projection failure after commit 和 concurrent rollback edit 在已定义 validation point 安全失败且提交语义明确；quarantine relocation/revalidation 保留观测到的 replacement，并把 §12.2 最终 pathname deletion gap 记录为 residual limitation。Windows 对任何无法保证无 privilege 精确重建的 pre-existing symbolic link 在 mutation 前 fail closed；该策略必须有不依赖创建 link 权限、不可 Skip 的 C assertion，native link fixture 另由 W 补充。
25. **Structured recovery `[C+S+W]`**：machine typed variants（含 ad-hoc、source refresh、current candidate choice）可确定展开为完整 resolved argv，无占位伪命令；display renderer 不改变 argv。
26. **Windows native `[W]`**：构建、doctor、Orient、source/outcome/answer/recovery 在 cmd.exe 与 PowerShell 捕获原 argv；覆盖空格、引号、Unicode、CR/LF、`% ! & ^`。
27. **GitHub limits/tool fallback `[G+H]`**：在宿主侧覆盖 100 sub-issues、50 dependencies、权限不足、`gh<2.94` 与 REST fallback；真实 API/fixture 证明平台语义，Claude/Codex/Pi transcript 证明生成能力实际执行相同 fallback/reconciliation。
28. **Ten adapters `[S+H]`**：install/list/doctor/refresh/uninstall 精确六能力文件树，所有模板共享 trust/publication/destructive 边界。
29. **Current-only ownership safety `[C+S]`**：只有当前 manifest 版本和精确 current ownership 可授权 mutation；v1/任意非当前、恶意或 malformed manifest、marker-only state、retired settings 与自定义 `/opt/custom/slipway` 一律不迁移、不推断、不删除用户文件/settings。
30. **Privacy minimization `[C+S]`**：raw body/comments/token 不进 journal；accepted sensitive text 的警告、最小权限、redaction 与 purge 文档一致。
31. **Legacy namespace coexistence `[C+S]`**：旧 `runtime/cache/scope-root/scopes/locks/processes/repair-backups` 与新 `runs/` 共存；Run 忽略旧数据，doctor 只给 advisory，install/refresh/uninstall 不迁移、不删除、不阻塞。
32. **Resume budget `[C+S]`**：显式 budget 替换余额；省略时保留正余额或在零余额恢复 `max(initial,3)`；candidate pause 不谎称已应用，fresh Orient 消耗一次。
33. **Machine JSON stability `[C+S+W]`**：所有 §7 public/hidden JSON 使用版本化 schema/golden；Linux 与 Windows 的字段、null/omission、exit/error code 一致。
34. **Bounded context `[C]`**：超长多语言 decisions/outcomes 的优先级、UTF-8 截断、omission marker 和 digest 确定；replay byte-identical，requirements 不被截断或替代。
35. **Action union completeness `[C+S]`**：schema 接受且只接受 scoped Implement authorization；拒绝其他 Action 上的字段与所有 Clarify partial 组合；§9 矩阵、Go types、examples 与 golden 同源。

---

# 19. 测试与证据参考

实现与发布按实际影响面和风险选择证据；下列项目提供推荐覆盖面，不是固定的全有或全无 release gate。CLI 不读取这些结果来决定 Run、Review、Issue 或交付状态：

- `gofmt`、`git diff --check`；
- `go test ./... -count=1`；
- autopilot/runstore/fsutil/adapter focused race tests；
- source parser/hash 的 fuzz/property/golden tests；
- Action/Outcome JSON Schema、合法组合、idempotency、stale Action 的 table tests；
- root/leaf namespace swap、same-UID TOCTOU、quarantine relocation/revalidation、tail repair 与 projection failure tests；另有不可 Skip 的 Windows all-symlink pre-mutation fail-closed policy test，native fixture 只作 W 补充；测试覆盖已定义 validation points，并显式保留 §12.2 的最终 pathname deletion residual limitation；
- `go vet`、testlint、golangci-lint 与架构依赖守卫；
- Windows amd64 与 Linux amd64 交叉编译；
- Windows native cmd/PowerShell argv/recovery suite；
- `acceptance/*.sh` 实际调用构建后二进制，不能只测试 helper JSON；
- 十宿主 adapter Shell 验收；
- host-side reproducible GitHub fault-injection harness + 隔离 GitHub.com User-owned fixture repository；该 harness 由 generated capabilities/宿主验收消费，不伪装成 CLI binary 的 deterministic Go contract；
- Claude、Codex、Pi prompt-level matrix，特别覆盖 publication timeout/partial/retry 与 grill-me 决策边界；transcript 必须脱敏；
- actionlint、yamllint、markdownlint、本地链接检查；
- website sync/build；
- Nix check/build；
- Docker Git-backed doctor/run/source smoke；
- GoReleaser check/snapshot、archive LICENSE 与 Homebrew/Scoop/AUR 安装 smoke。

Shell、fixtures、requirements matrix 和 sanitized transcripts 位于 `acceptance/`；不得把可执行验收资产放回 `scripts/`。需要 GitHub credential 的 live fixture 使用受保护测试账号/repository，不在 fork PR 暴露 secret；无凭据 CI 仍运行 host-side reproducible fault harness，但不把它冒充真实 GitHub 语义或模型行为证明。

---

# 20. 明确非目标与信任声明

- 不规定总体产品设计、功能地图或其存储路径；
- 不要求 `PRODUCT.md`；
- 不生成或维护 Spec、Delta、living requirements registry；
- 不要求 GitHub Project；
- 不依赖 Organization repository 功能；
- 不在 Go binary 中持有 GitHub token、联网验证 provider 或实现 Project/tracker runtime；
- 不承诺 GitHub Issue 创建 exactly-once；承诺 approved operation/item plan、对账、无盲重试和歧义报告；
- 不声称 CLI flag 能密码学证明人类存在；宿主是明确记录的 trusted attester；
- 不承诺 journal 绝不含敏感文本；承诺数据最小化、警告、权限、redaction 与明确 purge 边界；
- 不自动 claim、close、merge、archive Issue；
- 不从 Issue close、Project status、PR merge 或 Run ended 推断部署；
- 不创建、切换或删除 worktree；
- 不恢复旧 Change runtime 或任何兼容模式；
- 不自动生成 ADR、CONTEXT 或总体设计文档；
- 不把完整访谈 transcript、chain-of-thought、raw Issue body、完整 comments 或 credential 主动写入 journal；
- 不把无 marker 的普通 Issue 静默解释为 Change；用户可以确认规范化或使用 ad-hoc Run。

---

# English summary (non-normative)

The complete Chinese contract and versioned machine schemas above are authoritative; this section is navigational only.

```text
Objective Issue (optional planning parent; never executable)
  └── self-contained Change Issue (one independently deliverable issue-backed source)
        └── Run (one revision-pinned execution attempt)
```

Slipway is explicitly invoked, issue-driven but never GitHub-gated, interruptible, and non-certifying. Objectives have no runtime inheritance; decomposition materializes applicable requirements into each Change. There is no Spec, Delta, legacy Change runtime, gate, worktree binding, or completion certification.

The design requires neither GitHub Projects nor organization-only features. Ten adapters expose six capabilities; the CLI retains seven public commands. Hosts are declared trust attesters, Issue content is untrusted data, source revisions are pinned, destructive grants are scoped, and ad-hoc Runs remain available. The normative protocol, security, Windows, reconciliation, privacy, and evidence rules are defined above.
