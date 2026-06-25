# 为已有代码库接入 Slipway

本教程将带你在不改动应用行为的前提下，为一个已有仓库接入 Slipway。目标是在让 AI agent 着手规划功能开发之前，先建立可持续、有源码佐证的仓库上下文。

## 你将构建什么

你会在 `artifacts/codebase/` 下创建或刷新代码库地图，然后基于这份地图跑一个小规模的试点受管变更。

## 前置条件

- 一个可用的 `slipway` 二进制文件。
- 一个已有的 Git 仓库。
- 一个能读取文件并运行 Slipway CLI 的 AI 编码工具。

进入已有仓库：

```bash
cd path/to/existing-repo
git status --short --branch
```

如果仓库已经有未提交改动，先判断这些改动是否属于本次接入工作。无关的编辑请保留好。

## 第 1 步：初始化 Slipway

```bash
slipway init --tools codex
```

按你的团队实际情况填入适配器 ID：

```bash
slipway init --tools claude,codex,opencode
```

查看生成了哪些内容：

```bash
git status --short
```

## 第 2 步：构建基线代码库地图

```bash
slipway codebase-map --json
```

它会在以下目录创建可持续的、仓库范围的上下文：

```text
artifacts/codebase/
```

生成的基线是检测到的事实，而非最终撰写完成的分析。查看这些文件：

```bash
find artifacts/codebase -maxdepth 1 -type f | sort
```

## 第 3 步：让 AI 撰写有源码佐证的上下文

把下面这段提示词粘贴到你的 AI 编码工具里：

```text
Use Slipway's codebase-map instructions to refine artifacts/codebase/. Preserve
real baseline facts from `slipway codebase-map`, but add only source-backed
conventions and risks. Cite file paths for every convention. Do not refactor or
edit application code during onboarding.

Start with:
- slipway instructions stack --json
- slipway instructions architecture --json
- slipway instructions testing --json
- slipway instructions concerns --json
```

像审查代码一样审查结果。凡是无法对应到当前文件、测试、构建脚本、配置文件或现有文档的规则，一律删掉。

## 第 4 步：创建一个小规模试点变更

挑选最小但有价值、足以证明地图确有帮助的变更。合适的试点包括：

- 为某个已有的辅助函数补一个缺失的测试。
- 更新一篇文档页，使其与当前命令保持一致。
- 修一个有明确复现步骤的小 bug。
- 仅当仓库的路由和测试模式已经清晰时，再考虑新增一个健康检查端点。

创建受管变更：

```bash
slipway new "pilot change using the codebase map" --preset standard
```

查看交接信息：

```bash
slipway next --json --diagnostics
```

`input_context.codebase_map_status` 字段会告诉你 Slipway 当前如何看待这份地图：是 missing、scaffold-only、baseline、partial 还是 populated。如果它只是基线状态，而任务又依赖于约定，那就先停下来完善地图，再做规划。

## 第 5 步：在带有地图上下文的情况下规划

粘贴这段提示词：

```text
Continue the active Slipway change. During intake and planning, use
artifacts/codebase/ as advisory repo context. Do not invent conventions that are
not in the map or supported by current files. Keep the pilot small enough that
one task can verify whether the map improved planning.
```

每次交接之后：

```bash
slipway validate --json
slipway next --json --diagnostics
```

如果某个规划技能警告代码库地图缺失或仅为基线状态，就判断是要丰富地图，还是收窄任务。不要假设 AI 还记得上次会话里的仓库情况，然后径直往下走。

## 第 6 步：执行并审查试点

让 Slipway 来驱动实现和审查：

```bash
slipway run --json --diagnostics
```

当实现推进到任务执行器时，使用这段提示词：

```text
Execute the active Slipway task using the codebase map as context. Touch only
the target files declared in tasks.md. Run the task's verification command. If
the map contradicts current source, stop and report the discrepancy instead of
guessing.
```

实现完成后：

```bash
git diff --stat
slipway validate --json
slipway next --json --diagnostics
```

审查发现的问题应当通过 `slipway fix --json` 来修复，不要把审查和修复混在同一个上下文里。

## 第 7 步：沉淀有用的经验

如果试点暴露出某条值得长期保留的约定，就用有源码佐证的措辞更新 `artifacts/codebase/` 下对应的文件。保持表述精确：

- 好的写法：“HTTP 路由测试在 `internal/http/*_test.go` 中使用 `httptest.NewRecorder`。”
- 不好的写法：“始终编写全面的测试。”

跑一次最终的只读检查：

```bash
slipway validate --json
```

达到可完成状态后：

```bash
slipway done --json
```

把试点的 diff 和归档的受管记录一起提交。

## 你学到了什么

- `slipway codebase-map` 能为存量项目创建可持续的上下文。
- `slipway instructions <codebase-map-doc>` 是地图细化的撰写契约。
- 基线上下文有用，但有源码佐证、亲手撰写的上下文更可靠。
- 规划应当引用当前代码，而非臆想出来的约定。
- 在团队全面铺开之前，一个小试点就能验证地图是否真的有用。

## 相关内容

- [真实场景](../real-world-scenarios.md)
- [恢复与排障](../how-to/recover-and-troubleshoot.md)
- [设计](../explanation/design.md)
