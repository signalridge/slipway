# Slipway 中文文档

Slipway 是一个由用户显式调用、Issue 驱动但不被 GitHub 阻塞的 AI coding 软自动驾驶。中文页面是面向读者的导航与说明；完整的[中文产品契约](reference/product-contract.md)与版本化 [machine protocol schema](../reference/machine-protocol.schema.json) 才是实现权威。

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
            orient → clarify if needed → implement → review on observed diff → summarize
```

## 开始使用

- [从这里开始](start-here.md) — 从安装到完成一次 Run 的最短路径。
- [安装](installation.md) — 各平台安装方式与适配器命令。
- [产品权威](reference/product-overview.md) — 四轴模型、六项能力、七个命令。

## 参考

- [Issue 工作流](reference/issue-workflow.md) — Objective/Change Marker、label、自包含、GitHub 限制与发布流程。
- [命令参考](reference/commands.md) — 公开命令与 JSON 表面。
- [机器协议](reference/machine-protocol.md) — 版本化 Action / Outcome 契约与隐藏操作。
- [宿主适配器](reference/adapters.md) — 十个宿主、六项能力与 ownership safety。
- [Windows rendering 与 durability](reference/windows-rendering-and-durability.md) — argv rendering 与 crash durability。
- [验收与证据](reference/acceptance-evidence.md) — 证据类型与场景矩阵。

## 解释

- [架构](explanation/architecture.md) — package 布局与依赖方向。
- [运行日志与隐私](explanation/runs-and-privacy.md) — journal 内容、保留方式与隐私承诺。

## 决策与场景

- [架构决策](../decisions/0001-source-bundle-v2.md) — manifest-addressed source bundle。
- [Prompt 场景](../../acceptance/README.md) — 宿主行为评估。
