# 从这里开始

本指南从一个当前版本的 Slipway 二进制开始，完成一次由用户控制的 Run。

## 1. 确认 CLI 版本

本文档描述的接口有七个用户命令，外加机器协议：

```bash
./slipway --help
```

输出必须包含 `install`、`uninstall`、`list`、`doctor`、`run`、`status` 和 `stop`。若没有，说明已安装的软件包属于旧版本。请构建当前 checkout 或选择更新且兼容的 tag，详见[安装](installation.md)。

## 2. 安装一个宿主适配器

在 AI 宿主将要工作的 Git worktree 中执行：

```bash
./slipway install --tool claude
./slipway doctor
```

可将 `claude` 替换为 `codex`、`copilot`、`cursor`、`kilo`、`opencode`、`pi`、`qwen` 或 `windsurf`。Kiro 首次安装需要指定 surface：

```bash
./slipway install --tool kiro --surface ide   # 或：--surface cli
```

`install` 只写入宿主本地能力文件并记录 hash；不会修改全局宿主设置，也不会安装后台自动 hook。生成路径见[宿主适配器](reference/adapters.md)。

## 3. 主动启动 Run

在 AI 编程工具中，主动调用生成的 `slipway-run` 能力并给出一个任务，例如：

> 为 reports 命令增加 CSV 导出，并补充测试。

宿主向 CLI 请求一个 Action、执行它、提交结构化 Outcome，重复这一过程直到 Run 暂停或进入总结。它使用的 `protocol` 操作是公开且有文档的，但用户无需手工驱动：每次响应都已携带确切的下一条命令。

直接集成 CLI 的宿主可以这样启动同一个 ad-hoc Run：

```bash
./slipway run --json -- "为 reports 命令增加 CSV 导出"
```

该命令返回第一个 `orient` Action；CLI 本身不会修改代码。

## 4. 选择来源

| 来源 | 适用场景 |
| --- | --- |
| Ad hoc | 任务很小、私密、紧急、离线，或不需要 Issue。 |
| GitHub Change Issue | 需求需要一个长期、可审阅并按 revision 固定的来源。 |

对于 issue-backed Run，请通过生成的 `slipway-run` 能力传入 GitHub Change Issue。宿主负责获取 Issue、构造临时 source envelope，再交给 CLI。除非你正在开发宿主集成，否则不要手写 envelope。

Objective 可以组织多个 Change，但不能启动 Run。发布 managed Issue 前，请阅读 [GitHub Issue 工作流](guides/github-issues.md)。

## 5. 保持控制

Run 只会报告以下四种 pause reason 之一：

- 需要真正的人类决策——这也涵盖 Issue 来源变化或不可用的情况，它以决策形式暂停，而不是单独的一种原因；
- 环境依赖不可用；
- Action budget 耗尽；
- 需要确认精确的破坏性范围。

生成的宿主会展示可选响应。用户也可以随时 skip、stop、调整顺序或接管，无需说明理由。普通实现步骤不会重复索要授权。

常用检查命令：

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

`stop` 会保留恢复数据。Run ended 只表示 Slipway 没有更多自动 Action；测试、Review finding、仓库策略、合并审批和发布决策仍是彼此独立的事实。

## 6. 了解保存内容

Run 数据位于 `<git-common-dir>/slipway/runs/`，可能包含目标、已接受需求、用户回答、Outcome 和命令摘要。Slipway 不会主动收集 token、环境变量 dump、无关文件、完整对话或 hidden reasoning，但无法保证 journal 完全不含敏感信息。

处理敏感内容前，请阅读 [Run、恢复与隐私](guides/runs-and-recovery.md)。

## 继续阅读

- [核心概念](explanation/concepts.md)
- [命令参考](reference/commands.md)
- [宿主适配器](reference/adapters.md)
- [机器协议](reference/machine-protocol.md)
