# 命令参考

本页是 Slipway 命令的 Diataxis 参考入口。展开版操作参考仍然保留在
[命令](../commands.md)；本页让生成的命令面清单锚定在 `docs/reference/` 之下。

大多数路由命令在需要结构化输出时都支持 `--json`。
`slipway validate` 和 `slipway done` 不需要单独的 `--json` 标志就输出 JSON，
`slipway init` 仅用于初始化配置，而 `slipway config`、`slipway tool` 和
`slipway hook` 是公开的纯 CLI 命令面，不生成适配器 prompt 包装。

## 命令索引

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway new` | mutation | 创建一个受治理的变更。 |
| `slipway intake` | mutation | 执行意图澄清与授权。 |
| `slipway plan` | mutation | 编写或修订规划产物。 |
| `slipway implement` | mutation | 执行 S2 阶段的实现波次编排。 |
| `slipway review` | mutation | 执行 S3 阶段的评审收敛。 |
| `slipway fix` | mutation | 针对 S3 评审发现派发修复。 |
| `slipway done` | mutation | 归档一个已就绪可完成的变更。 |
| `slipway next` | query | 查看下一个技能或阻塞项，但不推进流程。 |
| `slipway run` | mutation | 驱动当前阶段直到触发某个停止条件。 |
| `slipway status` | query | 显示生命周期状态与下一步操作。 |
| `slipway codebase-map` | mutation | 创建或刷新仓库范围内的持久化上下文。 |
| `slipway handoff` | mutation | 写入或显示按变更划分的咨询性续作笔记。 |
| `slipway preset` | mutation | 确认或更改当前生效的预设。 |
| `slipway validate` | query | 重新计算就绪度，但不推进流程。 |
| `slipway abort` | mutation | 中止当前的执行会话。 |
| `slipway cancel` | mutation | 取消并归档一个活动中的变更。 |
| `slipway delete` | mutation | 丢弃已放弃的受治理本地状态。 |
| `slipway repair` | mutation | 执行有界范围的本地完整性修复。 |
| `slipway evidence` | mutation | 记录受支持的任务或技能证据。 |
| `slipway tool` | mutation | 运行生成技能所使用的纯 CLI 辅助工具。 |
| `slipway hook` | mutation | 运行生成的适配器配置所使用的纯 CLI 宿主 hook 辅助命令。 |
| `slipway health` | query | 显示仓库本地的完整性检查结果。 |
| `slipway instructions` | query | 显示产物或 codebase-map 的编写约定。 |
| `slipway init` | mutation | 初始化运行时布局与可选适配器。 |
| `slipway config` | mutation | 查看并设置仓库级配置键。 |

## JSON 命令面标记

以下示例保持原样，因为生成的命令面清单会检查每个 JSON 契约是否仍能在文档中找到。

```bash
slipway new --json
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway next --json
slipway run --json
slipway status --json
slipway handoff show --json
slipway validate
slipway done
slipway codebase-map --json
slipway preset <level> --json
slipway abort --json
slipway cancel --json
slipway delete --change <slug> --json
slipway repair --json
slipway evidence task --result-file task-result.json [--result-file next-task-result.json ...] --json
slipway evidence skill --skill <name> --verdict pass --json
slipway evidence skill --skill <name> --verdict pass --refresh-current --json
slipway health --json
slipway instructions <artifact> --json
slipway config list --json
```

当你需要查看阻塞项细节、产物就绪度细节、状态转换轨迹或上下文预算诊断时，
请在 `next` 或 `run` 上加 `--diagnostics`。

## 子命令与模式要点

- `slipway handoff write` 从 stdin 写入咨询性的续作笔记；bare 形式需要 pipe 一段完整的 `## Current Position` 正文，加 `--section <name>` 时会从 stdin 替换指定小节。
- `slipway handoff show --json` 以结构化形式输出当前变更的 handoff。
- `slipway evidence task --result-file <path> --json` 导入紧凑的执行任务结果；重复 `--result-file` 可进行原子批量导入。
- `slipway evidence skill --skill <name> --verdict pass --json` 在拥有该 skill 的阶段记录治理 skill 证据。
- `slipway evidence skill --skill <name> --verdict pass --refresh-current --json` 只用于把已选 S3 review skill 的当前 passing 证据作为一次明确重跑来替换；普通重复证据仍会被拒绝。
- `slipway status --stats --json` 报告工作区诊断，不重新引入已退休的顶层 `stats` 命令。
- `slipway health --doctor --json` 在 health 报告中加入面向修复的诊断。
- `slipway tool <helper>` 由生成的 skill 直接调用，不生成适配器提示词接入面。
- `slipway hook session-start` 和 `slipway hook context-pressure` 由生成的宿主 hook 配置直接调用；hook 辅助命令会静默失败，避免自动 hook 阻塞用户。
- `slipway config`、`slipway config list --json`、`slipway config list --env [--json]`、`slipway config get <key> --json` 和 `slipway config set <key> <value>` 用于查看 `.slipway.yaml` 配置键和运行时/密钥环境变量面；只有文件配置键可以更新。`config` 刻意保持 CLI-only，不生成适配器提示词接入面。

## 只读类命令

以下命令只检查状态，不改变生命周期的权威记录：

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
```

在选择某个会产生变更的命令前，先用它们查看现状。

## 改变阶段状态的命令

以下命令可能推进或改变受治理的状态：

```bash
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway done
slipway run --json --diagnostics
```

如果某个变更命令失败即停，请重新运行当前的只读检查，
并按提示给出的恢复命令操作。

配置项 `execution.auto` 作用于 `intake`、`plan` 和 `implement`。
这些阶段命令本身不接受按次调用的 `--auto` 或 `--no-auto` 覆盖标志；
当需要单次运行的覆盖行为时，请使用 `slipway run --auto` 或 `slipway run --no-auto`。

## Run 自动模式

`slipway run` 可以自动越过纯节奏性的暂停，让受治理的变更持续推进，
而不必在每次例行交接时都需要人工重新停下确认。你可以用 `execution.auto`
配置项按仓库启用它，也可以为单次调用覆盖它：

```bash
slipway run --json --auto
slipway run --json --no-auto
```

对那一次运行而言，`--auto` 和 `--no-auto` 的优先级高于 `execution.auto` 配置。
在自动模式下，Slipway 会基于先前的授权，自动越过纯节奏性的暂停（不含
`security-review` 的评审批次，以及非敏感、非 security-review 的技能交接），
并自动确认待处理的工作流预设升级（仅升级，绝不降级）。而 `security-review`
边界、敏感与护栏确认、intake 的已批准摘要（Approved Summary），以及每一道证据门，
仍会硬性停下，永远不会被自动越过。

## 命令面清单

`docs/SURFACE-MANIFEST.json` 由 Slipway 自有的 Go 权威源重新生成：

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

新增或修改命令、JSON 输出契约或面向文档的命令面时，
请确保其标记仍出现在清单对应行的 `docs` 文件中。

## 完整细节

详细的命令参考仍保留在 [命令](../commands.md)，涵盖创建选项、查询类命令、诊断、
输出标志，以及常见的 JSON 调用方式。
