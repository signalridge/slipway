# 第一个受治理的变更

本教程会带你用 Slipway 跑通一个仅涉及文档的小改动。重点不在于这次 README 编辑本身，而是借此看清 Slipway 如何呈现生命周期状态、skill 交接、证据、评审，以及失败即停（fail-closed）的恢复机制。

## 你将构建什么

你会往一个用完即弃的 README 里加上一个简短的 "Usage" 章节。

## 前置条件

- 一个可用的 `slipway` 二进制文件。
- 已安装 Git。
- 一个能读取 Slipway 生成的 skill 或命令界面的 AI 编程工具，例如 Codex、Claude、Cursor 或 OpenCode。

如果你用的是 Slipway 源码 checkout，而不是已安装的二进制，请在该 checkout 目录下把 `slipway` 替换成 `go run .`。

## 第 1 步：创建一个教程仓库

用一个用完即弃的目录：

```bash
mkdir slipway-first-change
cd slipway-first-change
git init
printf '# Slipway First Change\n\nA tiny repo for trying Slipway.\n' > README.md
git add README.md
git commit -m "chore: initial readme"
```

## 第 2 步：初始化 Slipway

选择与你的 AI 工具对应的适配器：

```bash
slipway init --tools codex
```

其他示例：

```bash
slipway init --tools claude
slipway init --tools opencode
slipway init --tools all
```

查看仓库状态：

```bash
git status --short
```

只有当你的团队希望把 `.slipway.yaml` 和生成的适配器纳入版本管理时，才提交它们。就本教程而言，在你检查这些生成文件之前，让它们保持未暂存（unstaged）即可。

## 第 3 步：创建一个受治理的变更

```bash
slipway new "add a small README usage note" --profile docs --preset standard
```

查看当前活动的变更：

```bash
slipway status --json
slipway next --json --diagnostics
```

把 JSON 当作当前的权威来源来读。它会告诉你下一个要运行的 skill、blocker 或命令。不要凭记忆去推断下一个阶段。

## 第 4 步：让 AI 编写 intake

在教程仓库里，把下面这段粘贴进你的 AI 编程工具：

```text
Use the active Slipway change. Inspect `slipway next --json --diagnostics`.
Complete only the surfaced intake or artifact-authoring handoff. The objective
is to add one README Usage section later; do not edit README.md during intake.
Do not edit change.yaml, lifecycle events, verification records, or runtime
evidence by hand.
```

当 AI 报告交接已完成时，再次查看：

```bash
slipway status --json
slipway next --json --diagnostics
```

如果 Slipway 报告缺少某个 artifact，就运行它指明的命令。例如：

```bash
slipway instructions requirements --json
```

instructions 命令给出的是编写契约。AI 必须写出真正的 artifact 内容；直接照抄占位模板会被各道 gate 有意拒绝。

## 第 5 步：运行规划

通过 CLI 界面推进规划：

```bash
slipway run --json --diagnostics
```

如果这一步返回了另一个 skill 交接，就粘贴这段提示：

```text
Continue the active Slipway change from the current `slipway next --json`
handoff. Author only the required planning artifact. Keep the eventual
implementation scoped to README.md. If the task plan needs target files, use
README.md only.
```

每次交接之后，重复这套只读检查：

```bash
slipway validate
slipway next --json --diagnostics
```

只有在 plan-audit 各道 gate 通过之后，规划才算可以进入实现阶段。如果 Slipway 失败即停，就按它指明的产物或评审恢复路径来处理。不要跳过规划这道 gate。

## 第 6 步：实现 README 改动

当 Slipway 进入实现阶段时，粘贴这段提示：

```text
Execute the active Slipway implementation handoff. Change only README.md. Add a
short Usage section with a command example that tells readers to run
`slipway status --json` before relying on lifecycle state. Run any targeted
verification command named by the task. Record task evidence only through the
Slipway command or generated execution skill that owns task evidence.
```

预期的 README 形态很小：

````markdown
## Usage

Inspect the current governed state before acting:

```bash
slipway status --json
```
````

AI 完成后，检查差异：

```bash
git diff -- README.md
slipway validate
slipway next --json --diagnostics
```

如果校验报告 `scope_contract_drift`，说明改动碰到了任务 `target_files` 之外的文件。请通过 Slipway 给出的路径修复 scope 或修订 plan；不要把这个文件藏进证据里。

## 第 7 步：评审并收尾

让 Slipway 运行评审：

```bash
slipway run --json --diagnostics
```

如果选定的评审证据缺失或过期，就重新运行由 `next --json --diagnostics` 指明的那个评审器。如果评审发现了问题，使用：

```bash
slipway fix --json
```

把返回的修复契约交给一个全新的 AI 上下文。修复完成后，重新运行受影响的评审器。

当状态报告 done-ready 时：

```bash
slipway done
```

然后查看有哪些东西发生了变化：

```bash
git status --short
find artifacts/changes -maxdepth 3 -type f | sort
```

如果这是一次真实的工作，就把 README 和归档的 Slipway 记录一起提交。

## 你学到了什么

- `status`、`next` 和 `validate` 是只读的权威检查。
- `run` 只会推进到下一个 skill、blocker 或 done-ready 状态为止。
- artifact 是从 `slipway instructions` 编写出来的，而不是从模板照抄。
- 实现的 scope 来自 `tasks.md` 里的 target files。
- 过期的证据要靠重新运行对应的阶段或评审器来修复。
- `done` 只会在达到受治理的就绪状态之后才归档变更。

## 相关内容

- [从这里开始](../start-here.md)
- [命令](../reference/commands.md)
- [工作流](../explanation/workflow.md)
