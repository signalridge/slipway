# Slipway 中文文档

Slipway 为 AI 编程宿主增加一套小而由用户控制的工作流。先从你要完成的任务开始；只有在集成或维护 Slipway 时，才需要阅读机器协议和架构页面。

[English](../en/index.md) · [日本語](../ja/index.md)

## 开始使用

- [从这里开始](start-here.md)——构建或安装 Slipway、添加一个宿主适配器并执行一个任务。
- [安装](installation.md)——版本兼容、软件包、源码构建、升级与卸载。
- [核心概念](explanation/concepts.md)——Run、Action、来源、Objective、Change 与结束语义。

## 指南

- [GitHub Issues](guides/github-issues.md)——何时使用 Objective 或 Change，以及 issue-backed Run 如何工作。
- [Run、恢复与隐私](guides/runs-and-recovery.md)——检查、停止、恢复、保留或删除 Run。
- [机器协议 v2 教程](guides/machine-protocol-v2.md)——使用严格 Outcome 完整执行一次宿主集成生命周期。

## 参考

- [命令](reference/commands.md)——七个用户命令、generated adapter 调用的 `protocol` 操作及其 flags。
- [宿主适配器](reference/adapters.md)——生成目标、调用方式与 ownership 安全。
- [机器协议](reference/machine-protocol.md)——宿主集成使用的版本化 JSON。

## 维护者

- [架构](explanation/architecture.md)——进程边界、代码包、存储与信任边界。
- [开发参考](contributing.md)——仓库布局和验证命令。
- [贡献指南](../../CONTRIBUTING.md)——Pull Request 工作流。
- [验收套件](../../tests/acceptance/README.md)——可执行与人工行为验证。
- [架构决策记录](../../adr/README.md)——历史技术理由，与用户文档分开保存。

英文、中文、日文三套页面描述同一产品。精确的机器字段形状位于与语言无关的 JSON Schema 中；任何一种翻译都不再充当独立产品契约。
