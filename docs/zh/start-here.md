# 从这里开始

本页给出从「我有一个仓库」到「Slipway 正在治理一个真实变更」的最短路径。

Slipway 围绕几类长期存在的项目记录构建：

| Slipway 界面 | 作用 |
| --- | --- |
| 受治理变更 | 位于 `artifacts/changes/<slug>/` 下的一个有边界的工作单元。 |
| 代码库地图 | 位于 `artifacts/codebase/` 下的共享仓库上下文，用于既有项目（brownfield）的工作。 |
| 任务证据 | 位于 `.git/slipway/runtime/changes/<slug>/evidence/` 下的运行时凭证。 |
| 评审证据 | 必须与当前 worktree 匹配的新鲜验证记录。 |
| AI 适配器 | 生成的宿主文件，把 agent 重新引导回 Slipway CLI。 |

CLI 是权威来源。AI 工具可以协助撰写工件、运行各个阶段，但不应臆造生命周期状态，也不应手工编辑证据。

## 选择你的路径

| 场景 | 从这里开始 |
| --- | --- |
| 你刚接触 Slipway，想做一次小型的端到端演练。 | [第一个受治理变更](tutorials/first-governed-change.md) |
| 你要把 Slipway 引入一个已有行为的仓库。 | [接入既有代码库](tutorials/onboarding-existing-codebase.md) |
| 你只需要安装和适配器相关的命令。 | [安装并刷新适配器](how-to/install-and-refresh-adapters.md) |
| 某个变更卡住了、过期了，或让人摸不着头脑。 | [恢复与排错](how-to/recover-and-troubleshoot.md) |
| 你在评估它的设计。 | [设计](explanation/design.md)与[工作流](explanation/workflow.md) |

需要具体的落地模式时，请参考[真实场景](real-world-scenarios.md)。

## 首次安装

挑选你的平台支持的安装方式。常见选项：

```bash
brew install --cask signalridge/tap/slipway
go install github.com/signalridge/slipway@latest
```

然后确认二进制文件可见：

```bash
slipway --help
```

完整的平台矩阵、发行包路径、校验和验证以及源码构建说明，仍在[安装](installation.md)中。

## 初始化仓库

在仓库根目录运行：

```bash
slipway init --tools codex
```

填入你实际使用的工具 ID：

```bash
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`slipway init` 会写入 `.slipway.yaml` 以及可选的、生成出来的 AI 工具适配器。适配器只是便利界面，CLI 始终是权威来源。

## 启动一个受治理变更

你不必手工驱动 Slipway。在你的 AI 工具会话里，用自然语言描述这个变更：

> 在 README 里加一段简短的使用说明。

`slipway init` 生成的适配器会把这个请求路由进受治理的生命周期。入口 skill 接手该变更，agent 替你跑完各个 `slipway` 阶段——意图采集（intake）、规划、实现、评审，以及收尾门（done gate）——只有当 Slipway 抛出 skill 交接、检查点、阻塞项或就绪可收尾（done-ready）状态、需要你介入时才会暂停。

判断变更是否真正完成的是 Slipway，而不是 agent。想查看 agent 正在读取的同一份状态时，使用只读界面：

```bash
slipway status --json
slipway next --json --diagnostics
```

### 自己手动驱动（可选）

如果你更愿意手工跑生命周期，同一个变更也可以通过普通命令来创建和推进：

```bash
slipway new "add a short usage note to README" --profile docs --preset standard
slipway run --json --diagnostics
```

`slipway run` 只在 Slipway 拥有的阶段之间推进，并在每个面向操作者的边界处停下。如果它返回了一个 skill 交接，请在你的 AI 工具里完成这次交接，然后重新运行只读命令再继续。

## 如果它失败即停（fail closed）

失败即停的输出是一项特性。它意味着 Slipway 发现证据缺失或过期，并指明了下一步安全动作。

按这个顺序操作：

```bash
slipway status --json
slipway validate --json
slipway next --json --diagnostics
slipway health --doctor --json
```

然后按它指明的恢复命令执行。不要手工编辑 `change.yaml`、验证 YAML、任务证据或生命周期时间戳。如果证据过期了，重新运行其所属的阶段、评审者或任务证据路径，让 Slipway 能够从当前 worktree 重新推导出新鲜度。

## 继续前进

- 按照[第一个受治理变更](tutorials/first-governed-change.md)做一次可直接复制粘贴的首跑。
- 当仓库已有 agent 必须学习的约定时，按照[接入既有代码库](tutorials/onboarding-existing-codebase.md)操作。
- 需要确切的命令和 JSON 界面细节时，使用[命令](reference/commands.md)。
