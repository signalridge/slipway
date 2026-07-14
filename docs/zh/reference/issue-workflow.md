# Issue 工作流

本页是便于操作的本地化指南；规范细节以[中文产品契约](product-contract.md)和版本化 [machine schema](../../reference/machine-protocol.schema.json) 为准。

只有确实需要多个独立交付时才建 Objective；Change 是唯一 issue-backed Run source。每个 Change 必须自包含全部有效执行 Requirements，父正文和 comments 不产生运行时继承。微小、敏感、紧急、GitHub 不可用或用户不愿建 Issue 时使用：

```bash
slipway run --budget N --json --root ABSOLUTE_ROOT -- GOAL
```

## Marker、label 与 body

Objective body 起始布局必须逐字为：`<!-- slipway-level: objective/v1 -->`，下一行 `<!-- slipway-publication-operation: UUID -->`，再下一行 `<!-- slipway-publication-item: UUID -->`。Title 为 `[Objective] ...`，labels 恰好 `level:objective` 加一个 `kind:*`。H2 为 Problem、Outcome、Requirements、Shared constraints、Non-goals、Changes。

Change draft 不含 `change/v2` marker 或 manifest，body 只有 `<!-- slipway-publication-operation: UUID -->` 和 `<!-- slipway-publication-item: UUID -->` 两个 receipt markers。Final Change 首行是 `<!-- slipway-level: change/v2 -->`，随后唯一严格 `slipway-manifest` JSON fence，并在 fence 后保留同一 operation/item markers。Manifest 有序数组将每个 chapter 的 stable key、role、title 绑定到 GitHub comment node ID、database ID hint 和完整 body digest。每个被引用 comment 第一非空行是 `<!-- slipway-section:v1 key=KEY -->`；其后是该章的 exact normalized Markdown。Profile 至少覆盖 Outcome、Requirements、Acceptance examples、Constraints、Non-goals 五类 role，并允许一类拆成多章。

Title 为 `[Change] ...`，labels 恰好 `level:change` 加一个 `kind:*`，可选 `ready-for-agent`。Manifest 是唯一 authority；comment 显示顺序、时间戳和未引用讨论都不参与。缺失、额外、重复、跨 Issue、字段不一致、minimized、被原地编辑或 hash 不匹配的章节 fail closed。普通无 marker Issue 必须让用户三选一：手工规范化、另建确认过的 linked Change、或 bounded ad-hoc Run。

## 自包含、关系与限制

Decompose 把所有适用 Objective requirements/constraints 物化为 child 的 manifest-addressed chapters；Kind 不继承；讨论决定须发布为 replacement chapter 并进入新 manifest。Objective→Change 使用一层 native sub-issues，允许恰好达到 100 children；Change blocked-by 的 blocking 与 blocked-by 两个方向独立，各允许恰好达到 50。只有本次写入会超过限制时才停止并报告，绝不把 overflow 转成 prose 或 gate。

探测 `gh --version`：`gh >= 2.94.0` 用一等关系操作，否则用已有认证调用官方 REST API，或报告 `environment_unavailable`；不造本地权威。Transfer 只信同一 `github.com` redirect，随后重取 repository/Issue node IDs、labels、parent、dependencies、canonical URL，保留旧 URL alias，并继续 marker/revision 比较；跨 host 不信任。

## 发布与对账

Objective 是单阶段发布：一次 preview 展示 exact title、完整 body、labels、relations、operation UUID 与 item UUID；对这些精确外写取得一次当前确认；refetch mutable targets；通过 `--body-file` 创建；歧义时按精确 marker pair 使用 paginated non-search API 对账；最后完整 readback identity、URL、title/body/digest、markers、labels 与 relations。Objective 不创建 chapter comments/manifest，不要求第二次 commit confirmation，也不进入 Implement。

Change 因远端 comment ID 在创建前不存在，仍采用两阶段确认。第一阶段展示完整 chapter drafts、同一 operation UUID、稳定 item UUID、body digests、预期 section order/roles、精确 labels/relations 和 expected parent revision，确认创建非权威 drafts。新 Change draft 只有 receipt markers、无 `change/v2` marker；amendment 不改 accepted body。创建并 refetch 验证 comments 后，展示含真实 IDs 的 exact final manifest。每个 amendment manifest（包括 content-identical replacement）都必须把 preview 时确认的 pinned revision 写入 `parent_requirements_revision`；initial manifest 省略该字段。取得第二次当前确认后，commit 前立即重取 head 并拒绝 parent drift，最后更新 Issue body marker/manifest；同一 receipt markers 保留在 manifest fence 之后。未引用 comments 始终只是 drafts；accepted chapter 不原地编辑，也不静默删除 abandoned drafts。

GitHub 没有 exactly-once create 或 body CAS。Timeout-after-success、partial relation failure、duplicate marker、index delay 或歧义时，用 paginated non-search API 对账，每个 item/label/relation 报 `created|matched|failed|ambiguous`；不盲重试或 rollback success。零匹配要重新 preview/确认，一个可收敛，多个暂停。

公开仓库 Issue 没有 private switch。写前警告 Requirements、goal、answers 与 command summaries 可能敏感。使用 private repository；仅实际漏洞用已启用的 private vulnerability reporting；也可用现有安全通道或 ad-hoc Run。识别到 credential 时脱敏值但保留真实命令身份；不收 token、未引用讨论、env dump、完整 transcript 或 hidden reasoning。Source import 只临时获取精确 Issue body 与 manifest 引用的 raw comment 字段，raw envelope 仅交给 CLI consume，随后删除临时文件；只持久化 accepted normalized materials 与 bounded catalog/provenance。
