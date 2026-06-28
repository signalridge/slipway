# 真实场景

用这一页为手头的工作挑选合适的 Slipway 路径。每个场景都遵循同一条规则：依靠当前 worktree 的证据往前推进，而不是凭记忆或手动改状态。

## 场景索引

| 场景 | 适用情形 | Slipway 的主要价值 |
| --- | --- | --- |
| 1. 第一个受治理的改动 | 想拿一个安全的小改动来熟悉生命周期。 | 完整走一遍证据闭环。 |
| 2. 接管一个已有项目 | 仓库已经有约定和风险区域。 | 在规划之前把真实的代码库上下文沉淀下来。 |
| 3. 交付一个产品功能 | 工作涉及代码、测试、文档和评审。 | 让范围、任务、证据和评审保持一致。 |
| 4. 修复评审发现的问题 | S3 评审发现了需要处理的问题。 | 通过全新上下文的修复把问题集中解决。 |
| 5. 恢复陈旧或卡住的改动 | 证据、任务或产物出现了漂移。 | 失败即停，并给出具名的恢复命令。 |
| 6. 向团队推广 adapter | 多个 AI 工具需要同一套 Slipway 界面。 | 由单一 CLI 权威生成宿主文件。 |

## 1. 第一个受治理的改动

想用低风险的方式熟悉生命周期时用它。

给 AI 编码工具的起始提示词：

```text
Use Slipway for one small docs-only change. Keep the scope to README.md,
inspect status and next before each mutating command, and stop if Slipway
reports stale evidence or out-of-scope files.
```

工作流程：

1. 用 `slipway init --tools <tool-id>` 初始化 adapter。
2. 用 `slipway new "add a short README usage note" --profile docs` 创建改动。
3. 用 `slipway next --json --diagnostics` 查看当前的 handoff。
4. 让返回的 skill 来撰写所需的产物或完成实现步骤。
5. 实现完成后运行 `slipway validate`。
6. 仅在状态已经 done-ready 后才运行 `slipway done`。

完成的标志：

- 目标文件确实改了。
- 产物包说明了改动的原因。
- 当前的校验认可这些证据。
- `done` 之后存在归档记录。

## 2. 接管一个已有项目

当代码库已有真实行为，但约定散落在源码模式、旧 PR、零散文档或评审者记忆里时用它。

起始提示词：

```text
This is an existing repo. Do not refactor yet. Use Slipway to create or refresh
the codebase map, then identify the smallest governed change that would prove
the map is useful. Cite files for every convention you record.
```

工作流程：

1. 运行 `slipway init --tools <tool-id>`。
2. 运行 `slipway codebase-map --json`。
3. 用 `slipway instructions stack`、`slipway instructions architecture`、`slipway instructions testing` 以及其他 codebase-map 指令主题，撰写或完善 `artifacts/codebase/` 下的文档。
4. 创建一个小的受治理试点改动。
5. 规划期间，确认 `next --json` 在 `input_context.codebase_map_status` 中报告了 map 的状态。
6. 复盘试点结果，只用有源码支撑的发现来更新 map。

护栏：

- 只记录当前文件能支撑的约定。
- 删除那些无法追溯到代码或文档的臆测规则。
- 在 map 写入实质内容之前，把只有基线的 map 当作仅供参考。
- 不要把大范围清理纳入接管任务。

完成的标志：

- `artifacts/codebase/` 包含经过复核的上下文。
- 第一个受治理的试点用上了这些上下文。
- map 没有沦为堆放臆测的垃圾场。

## 3. 交付一个产品功能

当工作包含实现、测试、文档和评审要求时用它。

起始提示词：

```text
Use Slipway for this feature. First clarify scope and acceptance criteria. Keep
target files explicit in tasks.md, run targeted tests for each task, and treat
review findings as a separate S3 repair batch.
```

工作流程：

1. 用 `slipway new "<feature>" --preset standard` 创建改动。
2. 让 intake 和规划产出真实的 `intent.md`、`requirements.md`、`decision.md`、`research.md` 和 `tasks.md`。
3. 确认每个任务都有具体的 `target_files`。
4. 通过 `slipway implement --json` 或 `slipway run --json` 执行。
5. 沿生成的 wave 执行路径记录任务证据。
6. 运行 S3 评审，仅在选定评审者的证据当前有效时才收尾。

完成的标志：

- 需求能对应到实现和测试。
- 任务证据与当前运行版本一致。
- 选定的评审和收尾证据都通过。
- `done` 在不掩盖脏改动的前提下归档了改动。

## 4. 修复评审发现的问题

当 S3 评审报告了需要处理的问题时用它。

起始提示词：

```text
Use Slipway fix for the selected review findings. First consolidate confirmed
findings by root cause. Make one repair pass, rerun the affected reviewers, and
do not repair findings inline while review is still reporting.
```

工作流程：

1. 查看 `slipway review --json` 或 `slipway next --json --diagnostics`。
2. 运行 `slipway fix --json`。
3. 把返回的修复契约交给一个全新上下文的修复 agent。
4. 重跑受影响的选定评审者。
5. 仅在修复和评审的 context-origin 证据都当前有效后才继续评审。

完成的标志：

- 修复从根因上解决了选定的问题。
- 修复之后刷新了评审证据。
- 没有任何陈旧的选定评审者被悄悄忽略。

## 5. 恢复陈旧或卡住的改动

当 `next`、`status` 或 `validate` 报告证据陈旧、缺少任务凭证、范围漂移或本地状态不一致时用它。

起始提示词：

```text
Diagnose this Slipway blocker without editing state by hand. Run status,
validate, next with diagnostics, and health doctor. Follow only the named safe
recovery command or explain why none applies.
```

工作流程：

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
slipway health --doctor --json
```

如果 health 给出了一个有界的本地修复，运行：

```bash
slipway repair --json
```

如果某个阶段或评审者陈旧了，重跑它所属的阶段或评审者。如果某个产物缺少实质内容，运行 `slipway instructions <artifact>` 并撰写真实的产物。

完成的标志：

- 当前 worktree 的原始 blocker 已经消失。
- 时效性由所属命令或 skill 重新生成。
- 恢复过程没有伪造时间戳、判定结果或生命周期状态。

## 6. 向团队推广 adapter

当多人或多个工具需要同一套命令与 skill 界面时用它。

起始提示词：

```text
为该仓库实际使用的工具刷新 Slipway 适配器。保留生成目录附近的用户自有文件，
检查差异，不要让生成的工具文件凌驾于 CLI 权威之上。
```

工作流程：

```bash
slipway init --tools claude,codex,opencode
slipway init --refresh
```

只有当仓库确实有意支持 Slipway 生成的每一个 adapter 时，才使用 `--tools all --refresh`。

完成的标志：

- `.slipway.yaml` 反映了仓库的默认设置。
- 生成的 adapter 文件与当前 CLI 一致。
- 用户自有的宿主配置得到保留。
- 团队知道以 `slipway next`、`status` 和 `validate` 为权威来源。
