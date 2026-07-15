<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>
</p>

[文档](https://signalridge.github.io/slipway/zh/) ·
[从这里开始](docs/zh/start-here.md) ·
[安装](docs/zh/installation.md) ·
[版本记录](CHANGELOG.md)

[English](README.md) · **简体中文** · [日本語](README.ja.md)

</div>

# Slipway

Slipway 是一款由用户显式调用的 AI 编程软自动驾驶工具。它为 AI 编程宿主提供一套小而可恢复的工作流，同时把决策权和控制权留给用户。

一次 Run 每次只推进一个有界 Action：

```text
orient → 必要时 clarify → implement → 代码已变化且启用时 review → summarize
```

宿主负责实际工作；Slipway CLI 记录 Run、选择下一个 Action、独立观察仓库变化并提供结构化恢复。它不调用模型、不持有 GitHub token，也不替用户判断软件是否可以合并或发布。

> [!IMPORTANT]
> 请使用 `slipway --help` 中包含 `install`、`uninstall`、`list`、`doctor`、
> `run`、`status`、`stop` 的版本。包管理渠道可能落后于仓库；在包含这套接口的
> tag 发布之前，请从当前 checkout 构建。

## 为什么使用 Slipway？

- **显式启动：** 不会在后台或普通对话中自动触发。用户针对具体任务调用生成的能力或 CLI。
- **用户控制：** 可以随时 skip、stop、resume、调整顺序或接管，无需说明理由。
- **先查事实再提问：** 宿主先检查仓库，只有真正的产品决策才交给用户。
- **可恢复的 Run：** 追加式 journal 和固定的来源材料让 Run 无需依赖聊天记录也能恢复。
- **可选的 GitHub 来源：** 需要长期、可审阅来源时使用自包含 Change Issue；不需要、不适合或无法使用 Issue 时直接 ad hoc 启动。
- **如实输出：** 报告实际命令、退出结果、findings、known issues 和不确定性。Run ended 只表示 Action 队列为空。

## 从当前 checkout 快速开始

使用 [`go.mod`](go.mod) 声明的 Go 版本构建 Slipway，然后安装 AI 编程宿主适配器：

```bash
go build -o ./slipway .
./slipway install --tool claude
./slipway doctor
```

在 Claude 中显式调用生成的 `slipway-run` skill，并描述一个任务。其他宿主可将 `claude` 替换为：

```text
codex  copilot  cursor  kilo  kiro  opencode  pi  qwen  windsurf
```

Kiro 首次安装必须指定 surface：

```bash
./slipway install --tool kiro --surface ide   # 或：--surface cli
```

生成的能力会替你驱动机器协议。若你在直接集成宿主，可这样启动 ad-hoc Run：

```bash
./slipway run --json -- "为报表增加 CSV 导出"
```

该命令只返回第一个 Action，不会自行修改代码。完整交互见[入门指南](docs/zh/start-here.md)，集成细节见[机器协议](docs/zh/reference/machine-protocol.md)。

## 来源与 Run

| 来源 | 适用场景 | Slipway 保存什么 |
| --- | --- | --- |
| Ad hoc | 任务很小、私密、紧急、离线，或明确不想放到 GitHub。 | 目标和之后的 Run 事件。 |
| GitHub Change Issue | 任务需要可审阅、按 revision 固定的需求来源。 | 稳定的 Issue 身份、有界章节目录，以及按 digest 保存的已接受章节。 |

GitHub Objective Issue 可以组织多个 Change，但只有自包含 Change 能启动 issue-backed Run。来源格式不区分仓库属于个人账号还是 Organization。Slipway 不依赖 GitHub Projects、Organization 专属 Issue Types 或专属字段。

生成的 `propose` 和 `decompose` 能力可以协助准备 Issue。这些是宿主侧操作：宿主预览所有外部写入，使用用户自己的 GitHub 权限，并如实报告部分成功或失败。Run/source core 不获取或发布 GitHub 数据，也不保存凭据；独立的 `doctor` 命令可能调用用户本机的 `gh` 做只读诊断。

## 控制与恢复

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

Run 需要输入时，生成的宿主会给出精确的决策、来源选择、环境问题或破坏性范围。只有对应的显式响应后才继续。`stop` 保留恢复数据；删除 Run 目录会移除本地恢复能力，但不是安全擦除。

Run 数据位于仓库 Git common directory 下的 `<git-common-dir>/slipway/runs/`。其中可能包含目标、已接受需求、用户回答和命令摘要。Slipway 会尽量减少收集，但不能保证 journal 不含敏感信息，应将其视为本地私密数据。

处理敏感内容前，请阅读 [Run、恢复与隐私](docs/zh/guides/runs-and-recovery.md)。

## 文档

### 使用 Slipway

- [从这里开始](docs/zh/start-here.md)
- [安装](docs/zh/installation.md)
- [GitHub Issue 工作流](docs/zh/guides/github-issues.md)
- [Run、恢复与隐私](docs/zh/guides/runs-and-recovery.md)
- [核心概念](docs/zh/explanation/concepts.md)

### 查询精确接口

- [命令参考](docs/zh/reference/commands.md)
- [宿主适配器](docs/zh/reference/adapters.md)
- [机器协议](docs/zh/reference/machine-protocol.md)
- [架构](docs/zh/explanation/architecture.md)

### 参与开发

- [贡献指南](CONTRIBUTING.md)
- [开发参考](docs/zh/contributing.md)
- [验收套件](acceptance/README.md)
- [架构决策记录](adr/README.md)

## 许可证

Slipway 使用 [BSD 3-Clause License](LICENSE)。
