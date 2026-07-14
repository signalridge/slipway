# 产品权威（非规范性）

> **非规范性摘要。** 完整的[中文产品契约](product-contract.md)与版本化 [machine protocol schema](../../reference/machine-protocol.schema.json) 才是实现权威。本页用于导航，不构成第二份规格。

Slipway 是一个由用户显式调用、Issue 驱动但不被 GitHub 阻塞的 AI coding 软自动驾驶：

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
```

## Requirements 是临时的

Slipway 不维护 Spec、Delta Spec，也不维护永久的 Requirements registry。约束性原则是：

> Requirements 是临时交付契约，不是系统的永久模型。

开放 Issue 描述下一次期望发生的变化；Run 固定并执行其中一个 revision；交付后，代码、测试、用户文档、CI/policy 和运行行为接管当前事实。已关闭 Issue、关联 PR/commit 与 Run summary 保留历史原因，但不会成为当前系统的完整规格。GitHub `closed`、Project `Done`、PR `merged`、Run `ended` 和 deployment 是彼此不同的事实。

## 四个相互独立的轴

以下四个维度绝不能混为一谈。

| 维度 | 值 | 所有者 |
| --- | --- | --- |
| **Level** | `objective` / `change` | Issue body Marker（唯一权威）；label/title 只是 projection |
| **Kind** | `feature` / `bug` / `refactor` / `maintenance` / `research` / `docs` | repository label |
| **Requirements** | Outcome、Requirements、Acceptance examples、Constraints、Non-goals | Issue body |
| **Status** | Inbox、Clarifying、Ready、In progress、Done 等 | 人类/可选外部视图；绝不参与 Slipway 路由 |

Level 与 Kind 的笛卡尔积全部合法。Requirement 是 Objective/Change 的内容表达，不是颗粒度，也不是 GitHub Issue Type。正文第一个精确 Marker 是 Level 权威；label、title、`ready-for-agent`、Project 字段、测试结果、findings 和 Issue 状态都不得 gate 一个 Marker-valid Run。

## Objective 与 Change

只有一个结果确实需要多个可独立交付的 Change 时，才创建 Objective。Change 是唯一 issue-backed Run source，并且必须自包含：它产生一个连贯结果，可以独立 merge、验证和回滚，同时让仓库保持安全状态。`decompose` 会把 Objective 中所有适用的 requirement 与 constraint 物化到每个子项，使 Change 无需实时读取父级。父级 Kind 不会被继承；普通讨论 comments 不具权威性，除非它们被发布为 replacement chapter comment，并进入新的 manifest。

## 六项能力，七个命令

十个适配器精确生成六项由用户显式调用的能力：

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

`run` 是唯一软自动驾驶入口。`clarify` 无状态；`propose` 起草或发布经显式确认的托管 Issue；`decompose` 创建经确认的 Change 关系；`implement` 承担技术活动；`review` 只读并报告 Intent 与 Quality findings。系统不会生成 ambient session hook、prompt-submit hook、launcher、global router，也不会生成独立的技术验证能力。

CLI 精确公开七个命令：

```text
slipway install   install six host capabilities safely
slipway uninstall remove only pristine managed files
slipway list      show adapter installation state
slipway doctor    diagnose adapters, Git/GitHub capability, and recovery
slipway run       start an ad-hoc or issue-bound Run
slipway status    list or inspect recoverable Runs
slipway stop      stop without deleting the journal
```

不存在 `objective`、`change`、`issue`、`spec`、`plan`、`ticket`、`done`、`check` 或 `worktree` 命令。

## 固定 source 与不可信内容

Run 绝不信任可变的 `#42`，也不信任宿主自行生成的摘要。可信宿主只获取一次严格、由 manifest 寻址的 envelope；CLI 验证该 envelope，确定性解析有序 manifest，并使用 domain-separated digest 将每个章节固定到私有的 content-addressed material store。Journal、status、candidate 与 Action 只保留目录、provenance、byte count 和 revision，绝不保存 Markdown 或原始 Issue body。

宿主是声明的 trust attester；Issue 内容是不可信数据。Issue title、body、comment、label、link 和 attachment 都只是数据，绝不是 system 或 developer instruction。Issue 内的 prompt injection、credential request 和无关命令不具备宿主权限。

## Amendment 与破坏性权限

Issue-bound resume 必须且只能选择一种 source 模式：导入新的 envelope、显式沿用固定 snapshot，或用精确 ID 处理当前 candidate。存在实质变化的 candidate 会原子 void outstanding Action、queue 和 grant，并暂停等待显式选择；内容相同、仅替换 manifest 的版本则继续保留既有 answers。破坏性工作需要一次性、绑定 scope 的结构化 grant——自然语言“yes”绝不会授予权限；可信宿主只是 attester，不是人类在场的密码学证明。

由于 GitHub 既不提供 exactly-once Issue creation，也不提供 body compare-and-swap，发布过程使用经批准的 operation/item UUID Marker 与 reconciliation。Review 只报告 findings，不编辑代码，也不启动修复循环。

## 恢复与隐私

Git common directory 下的 append-only journal 是恢复权威；`run.json` 是可替换的 projection，`run.lock` 则串行化 journal mutation。Journal 可能包含已接受的 Requirements、goal、answer 和如实记录的 command summary。Slipway 会最小化数据，并对已识别的 credential 做脱敏，但不承诺 journal 绝不含 secret。删除 run directory 只会移除恢复能力，不代表安全擦除、清除备份或销毁密钥。

## 不提供完成认证

`ended` 只表示自动 Action queue 已空。Slipway 不认证正确性、交付、部署或发布就绪状态，也不认证不存在 findings。测试失败、未运行测试、Review findings、脏 worktree、缺少 ADR、label 和 Issue 状态，都不得 gate Run 的推进、release 或 merge。

继续阅读 [Issue 工作流](issue-workflow.md)、[命令参考](commands.md)、[机器协议](machine-protocol.md)、[Windows 行为](windows-rendering-and-durability.md)和[验收证据](acceptance-evidence.md)。
