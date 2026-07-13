<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

# Slipway

**由用户显式启动、Issue 驱动但不被 GitHub 阻塞的 AI 编程软自动驾驶。**

[English](README.md) · [日本語](README.ja.md) · [中文文档](docs/zh/start-here.md)

</div>

完整[中文产品契约](docs/zh/reference/product-contract.md)与版本化 [machine protocol schema](docs/reference/machine-protocol.schema.json) 是实现权威。

Slipway 协助 AI coding 宿主调查仓库、澄清真正需要人决定的问题、实施有界变化、按需只读 Review、恢复中断 Run 并报告事实。用户显式启动，随时可 skip、stop、resume 或接管。

```text
Objective Issue（可选 planning parent，不可执行）
  └─ 自包含 Change Issue（唯一 issue-backed source）
       └─ Run（固定 revision 的一次可中断尝试）
            orient → 必要时 clarify → implement → 观察到 diff 时 review → summarize
```

Requirements 是临时交付契约，不是永久 Spec。只有多个独立交付才建 Objective；每个 Change 自包含，不从 parent/comments 运行时继承。首个精确 body marker 是 Level 权威，label/title 只作 warning projection；`ready-for-agent`、Issue/Project status、测试和 findings 都不 gate marker-valid Run。

## 快速开始

```bash
go install github.com/signalridge/slipway@latest
cd 你的仓库
slipway install --tool claude

# Ad-hoc escape hatch：微小、敏感、紧急、GitHub 不可用或明确不建 Issue。
slipway run "给报表增加 CSV 导出" --json

# Issue-bound：可信宿主一次性获取严格、manifest-addressed Source Bundle v2。
slipway run "实施这个有界 Change" \
  --source-file /安全临时目录/change-envelope.json --json
```

CLI 不调用模型 provider、不持 GitHub token。宿主 attest GitHub fetch，但 Issue 内容仍是不可信数据；CLI 只接受 manifest 显式引用的 chapter comments，验证身份/digest，将 exact payload 固定为本地 material，并每次返回一个有界 Action catalog。宿主按 structured `_machine material` 操作逐章读取。Amendment 需显式 current-candidate choice；破坏性操作需精确 scope 的 one-shot structured grant，自然语言 yes 不授权。

[Issue 工作流](docs/zh/reference/issue-workflow.md)说明 Objective/Change marker、精确 Level/Kind labels、自包含、`gh >= 2.94`/官方 REST fallback、同 host transfer、100/50 限制、approved publication markers 与部分/歧义对账。

## 六个显式宿主能力

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

支持 Claude Code、Codex、GitHub Copilot、Cursor、Kilo Code、Kiro、OpenCode、Pi、Qwen Code、Windsurf，所有能力都需显式调用。Clarify 保留 Matt Pocock MIT `grill-me`/`grilling`：先查事实，按依赖一次一个决定并给推荐，理解改变后确认 shared understanding，默认无状态，wrap-up 立即停止；不提供隐式澄清文档化能力。Review 永远只读，不修复、不建 re-review loop。

## 七个公开命令

```text
install      安全安装六能力
uninstall    只删除 pristine 托管文件
list         查看 adapter 安装状态
doctor       诊断 adapter、Git/GitHub capability 与 recovery
run          启动 ad-hoc 或 issue-bound Run
status       列出/查看可恢复 Run
stop         保留 journal 并停止
```

隐藏的版本化 `_machine submit/answer/skip/resume/material` 见[机器协议](docs/zh/reference/machine-protocol.md)。`ended` 只表示自动 Action queue 为空，不认证正确、交付、部署、release-ready 或无 finding。

## Journal 与隐私

恢复权威为 `.git/slipway/runs/<run-id>/journal.jsonl`；`run.json` 可替换，`run.lock` 只串行 mutation。Journal 可能含 accepted Requirements、goal、answers 与如实 command summaries。Slipway 不承诺无 secret：它避免保存 raw body/comments、token、env dump、完整 transcript 与 hidden reasoning，并对识别到的 credential value 脱敏同时保留 command identity。Unix mode 与 Windows current-user ACL 仍有 root/admin、backup、malware、inherited ACL、同账户进程限制。

删除 run dir 只失去恢复能力，不是 secure erase、backup purge 或 key destruction。详见[运行日志与隐私](docs/zh/explanation/runs-and-privacy.md)、[Windows](docs/zh/reference/windows-rendering-and-durability.md)和[验收证据矩阵](tests/acceptance/README.md)。

Slipway 使用 [BSD 3-Clause License](LICENSE)。
