# 从这里开始

本页给出一条最短路径，带你从“我有一个代码仓库”走到“Slipway 正在我的控制下执行一项有边界的变更”。

Slipway 只在用户显式调用后启动，Issue 驱动但不被 GitHub 阻塞，也从不认证工作“已经完成”。工作通过一组精简、持久的表面流转：

| Slipway 表面 | 作用 |
| --- | --- |
| Objective Issue | 面向多个独立交付的可选规划父级。绝不可执行。 |
| Change Issue | 唯一 issue-backed Run source。自包含；承载全部有效 Requirements。 |
| Run | 位于 `.git/slipway/runs/<run-id>/` 下，一次固定 revision、可中断的执行尝试。 |
| 宿主能力 | 精确六项：`run`、`clarify`、`propose`、`decompose`、`implement`、`review`。 |
| 固定 source | 由 manifest 寻址并按 digest 固定的章节目录；绝不持久化原始正文。 |

CLI 是权威。宿主执行 Action；Slipway 负责调度、独立观察 Git，并保存恢复历史。宿主可以编写 Issue 草稿，也可以执行技术活动，但不应自行发明生命周期状态、手工编辑证据，或把 Issue 文本当作指令。

> 本页是非规范性指南。完整的[中文产品契约](reference/product-contract.md)与 [machine schema](../reference/machine-protocol.schema.json) 才是实现权威。

## 选择你的路径

| 场景 | 从这里开始 |
| --- | --- |
| 你刚接触 Slipway，希望完成一次小型端到端 Run。 | 先阅读 [Issue 工作流](reference/issue-workflow.md)，再执行下文的 `slipway run`。 |
| 你需要了解各平台与适配器命令。 | 阅读[安装](installation.md)和[宿主适配器](reference/adapters.md)。 |
| 某个 Run 已暂停、停止，或状态令人困惑。 | 阅读[命令参考](reference/commands.md)中的 `status`、`stop` 与 resume，以及[运行日志与隐私](explanation/runs-and-privacy.md)。 |
| 你正在评估产品设计。 | 阅读[产品权威](reference/product-overview.md)和[架构](explanation/architecture.md)。 |
| 你需要机器契约。 | 阅读[机器协议](reference/machine-protocol.md)。 |

## 安装并确认

根据你的平台，选择由官方 release 支持的安装方式：

| 平台 | 推荐方式 |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | 使用[安装](installation.md#linux-软件包)中列出的 `.deb`、`.rpm`、`.apk`、`tar.gz`、AUR 或容器镜像方式。 |
| Go 备用方式 | `go install github.com/signalridge/slipway@latest` |

然后确认命令已可用：

```bash
slipway --version
slipway doctor
```

完整的平台矩阵、release archive 路径、checksum 验证和源码构建说明仍集中在[安装](installation.md)中。

## 生成宿主能力

在任意 Git worktree 目录内运行以下命令，为你使用的宿主安装六项能力：

```bash
slipway install --tool claude
slipway install --tool codex,cursor,pi
slipway install --tool all
slipway install --tool kiro --surface ide   # or: --surface cli
```

未指定 `--tool` 时，Slipway 会为检测到的宿主目录安装相应适配器。`--refresh` 只更新 ownership hash 仍然匹配的文件。首次安装 Kiro 时必须指定 `--surface ide|cli`；后续 refresh 与 uninstall 会从已记录的信息推断对应表面。

## 启动一次 Run

工作以 Issue 为优先，但不被 Issue 阻塞。只有一项目标需要多个独立交付时才使用 Objective；Change 是唯一 issue-backed source，且必须承载全部有效 Requirements。

### Ad-hoc

对于微小、敏感、紧急、离线的工作，或你就是不想创建 Issue 的场景：

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### Issue-bound

可信宿主获取一次严格的 GitHub Change envelope，并把临时 raw envelope 传给 CLI：

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

Marker-valid 正文是 Level 权威；title/label drift 会产生警告，但不会形成 gate。CLI 验证 Issue 正文中的 manifest 及其精确引用的 comments，按 digest 固定每个章节，并且只保存有界的章节目录。读取本地 material 或恢复 Run 时，都不再需要该临时文件或 GitHub。

发布前请阅读 [Issue 工作流](reference/issue-workflow.md)。公开 Issue 没有 private 开关；敏感工作可能需要 private repository、适当的安全通道或 ad-hoc Run。

## 用户控制

Slipway 只会在你显式调用时启动。Run 获得授权后，每次推进一个版本化 Action；只有遇到真正的决策、source amendment、环境故障或破坏性确认时才会暂停。你无需为控制操作提供理由：

| 意图 | 结果 |
| --- | --- |
| **跳过这个** | 调用 outstanding Action 对应的精确 skip control。 |
| **停止** | 运行 `slipway stop`；保留 journal，Run 之后可以恢复。 |
| **接管** | 先停止，保留并报告 Run ID，且不执行 outstanding Action。 |
| **重排 / 先做 X** | 停止自动循环并交还控制；不会静默修改 queue，也绝不会把请求转换为 skip。 |

只有显式 resume 后，工作才会继续。宿主在提问前会先调查仓库事实。Clarify 遵循 [Matt Pocock `grill-me`](https://github.com/mattpocock/skills) 的对话纪律：按依赖顺序一次处理一个真正的人类决策，并给出建议与权衡；完整请求允许零问题；只有 grilling 改变了执行理解时才确认；用户要求 wrap-up 时立即以无状态方式停止。

Review 只读，报告 Intent/Quality findings，绝不启动修复循环。`ended` 只表示自动 queue 已空，不代表正确、已交付或已具备发布条件。

## 如果 Run 暂停

暂停是一项产品能力：Slipway 已到达需要你或其环境介入的位置。每次暂停和错误都会返回结构化 `next` 对象，其中包含有类型且可解析的 variants，而不是需要重新拼装的 shell string。

```bash
slipway status --json          # current state and the fresh derived next
slipway status <run-id> --json
```

随后执行指定的恢复 variant。Issue-bound resume 必须且只能选择一种 source 模式：导入新的 envelope、显式沿用固定 snapshot，或用精确 ID 处理当前 candidate。详见[机器协议](reference/machine-protocol.md)。

## 继续阅读

- [产品权威](reference/product-overview.md) — 四轴模型。
- [Issue 工作流](reference/issue-workflow.md) — Marker、label 与发布流程。
- [命令参考](reference/commands.md) — 七个公开命令。
- [宿主适配器](reference/adapters.md) — 十个宿主。
- [运行日志与隐私](explanation/runs-and-privacy.md) — journal 保存哪些内容。
