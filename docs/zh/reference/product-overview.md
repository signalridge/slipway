# 产品概览与权威入口

本地化网站使用本页作为共同导航入口。完整、规范性的实现权威是[中文产品契约](product-contract.md)，机器字段与合法组合由版本化 [machine protocol schema](../../reference/machine-protocol.schema.json) 约束。英文和日文 product overview 均为 non-normative summary。

```text
Objective Issue（可选且不可执行）
  └─ 自包含 Change Issue（唯一 issue-backed source）
       └─ Run（固定 revision 的一次可中断尝试）
```

Slipway 是 issue-first，不是 issue-gated；GitHub 不可用、敏感或微小工作仍可 ad-hoc Run。十个 adapter 精确生成六能力，CLI 精确公开七命令。Body marker 是 Level 权威，label/title 只是可确认修复的 projection。Run 可 skip/stop/resume；Review 只读且不自动修复；`ended` 只表示队列为空。

继续阅读[Issue 工作流](issue-workflow.md)、[Windows rendering 与 durability](windows-rendering-and-durability.md)和[验收证据](acceptance-evidence.md)。
