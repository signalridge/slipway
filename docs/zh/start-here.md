# 从这里开始

完整[中文产品契约](reference/product-contract.md)和版本化 [machine schema](../reference/machine-protocol.schema.json) 是实现权威。

Slipway 只在用户显式调用后组织 AI 编程宿主。它 issue-first 但不 issue-gated：

```text
Objective Issue（可选、不可执行）
  └─ 自包含 Change Issue
       └─ Run：orient → 必要时 clarify → implement → 观察到 diff 时 review → summarize
```

Change 是唯一 issue-backed source，必须自包含全部有效 Requirements。只有多个独立交付才需要 Objective。GitHub 不可用、敏感、微小、紧急或用户不想建 Issue 时直接 ad-hoc：

```bash
go install github.com/signalridge/slipway@latest
cd 你的-git-仓库
slipway install --tool claude
slipway run "给报表增加 CSV 导出" --json
```

Issue-bound Run 由可信宿主安全获取一次严格 Change envelope：

```bash
slipway run "实施这个有界 Change" --source-file /安全临时目录/change-envelope.json --json
```

Marker-valid body 是 Level 权威；title/label drift 只警告、不 gate。发布前阅读 [Issue 工作流](reference/issue-workflow.md)。公开 Issue 没有 private switch；敏感工作使用 private repo、适当 security channel 或 ad-hoc Run。

Agent 先查仓库事实。Clarify 采用 Matt Pocock `grill-me`：按依赖一次一个人的决定，给推荐与权衡；完整请求零问题；理解改变后确认 shared understanding；wrap-up 立即停止且不写文件。所有 Action 可无理由 skip，Run 可 stop/resume/接管。Review 只读报告 Intent/Quality，不修复、不建 loop。`ended` 只表示自动队列为空。

十个 adapter 精确生成六能力，CLI 精确公开七命令。Journal 可能含 accepted Requirements、goal、answers 与 command summaries；继续阅读[运行日志与隐私](explanation/runs-and-privacy.md)、[Windows](reference/windows-rendering-and-durability.md)和[验收证据](reference/acceptance-evidence.md)。
