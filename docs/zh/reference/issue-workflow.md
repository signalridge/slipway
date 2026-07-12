# Issue 工作流

本页是便于操作的本地化指南；规范细节以[中文产品契约](product-contract.md)和版本化 [machine schema](../../reference/machine-protocol.schema.json) 为准。

只有确实需要多个独立交付时才建 Objective；Change 是唯一 issue-backed Run source。每个 Change 必须自包含全部有效执行 Requirements，父正文和 comments 不产生运行时继承。微小、敏感、紧急、GitHub 不可用或用户不愿建 Issue 时使用：

```bash
slipway run "<ad-hoc goal>"
```

## Marker、label 与 body

Objective 的首个非空行必须逐字为 `<!-- slipway-level: objective/v1 -->`，title 为 `[Objective] ...`，labels 恰好 `level:objective` 加一个 `kind:*`。H2 为 Problem、Outcome、Requirements、Shared constraints、Non-goals、Changes。

Change 示例：

```markdown
<!-- slipway-level: change/v1 -->

## Outcome
一个可观察结果。

## Requirements
本次交付所需全部行为。

## Acceptance examples
可观察的具体例子。

## Constraints
产品与技术边界。

## Non-goals
明确不做什么。

## Implementation checklist
可选执行笔记；不进入 revisions。
```

Title 为 `[Change] ...`，labels 恰好 `level:change` 加一个 `kind:*`，可选 `ready-for-agent`。首个精确 marker 是 Level 权威；label/title drift 只警告，经确认才修复，不能阻塞 marker-valid Run。缺失、冲突、Objective 或未知 marker 都不能成为 Change source。普通无 marker Issue 必须让用户三选一：手工规范化、另建确认过的 linked Change、或 bounded ad-hoc Run。

## 自包含、关系与限制

Decompose 把所有适用 Objective requirements/constraints 物化到 child；Kind 不继承；comments 中决定须先折回 Change body。Objective→Change 用一层 native sub-issues，最多 100；Change blocked-by 每方向最多 50。到限就停止并报告，不用 prose 隐藏 overflow。

探测 `gh --version`：`gh >= 2.94.0` 用一等关系操作，否则用已有认证调用官方 REST API，或报告 `environment_unavailable`；不造本地权威。Transfer 只信同一 `github.com` redirect，随后重取 repository/Issue node IDs、labels、parent、dependencies、canonical URL，保留旧 URL alias，并继续 marker/revision 比较；跨 host 不信任。

## 发布与对账

写前展示完整 drafts 与 operation UUID、stable item UUID、repository、canonical body SHA-256、精确 labels/relations、expected revisions 的 plan，并确认该精确范围。Typed operation/item marker 紧跟 level marker；使用 body file，mutation 前 refetch，写后 readback。

GitHub 没有 exactly-once create 或 body CAS。Timeout-after-success、partial relation failure、duplicate marker、index delay 或歧义时，用 paginated non-search API 对账，每个 item/label/relation 报 `created|matched|failed|ambiguous`；不盲重试或 rollback success。零匹配要重新 preview/确认，一个可收敛，多个暂停。

公开仓库 Issue 没有 private switch。写前警告 Requirements、goal、answers 与 command summaries 可能敏感。使用 private repository；仅实际漏洞用已启用的 private vulnerability reporting；也可用现有安全通道或 ad-hoc Run。识别到 credential 时脱敏值但保留真实命令身份；不收 token、raw comments、env dump、完整 transcript 或 hidden reasoning。
