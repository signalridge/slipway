<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/zh/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/zh/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/zh/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[Documentation](https://signalridge.github.io/slipway/) |
[Start Here](docs/zh/start-here.md) |
[Quick Start](#quick-start) |
[Installation](docs/zh/installation.md) |
[Release Notes](CHANGELOG.md)

<br/>

**English** · [简体中文](README.zh.md) · [日本語](README.ja.md)

</div>

# Slipway

**一款由用户显式调用、Issue 驱动但不被 GitHub 阻塞的 AI 编程软自动驾驶。
代码由宿主编写；Slipway 调度有界工作、固定来源，并如实报告事实，而不认证
“完成”。**

> **英文版仅为非规范性摘要。** 完整的
> [中文产品契约](docs/zh/reference/product-contract.md)与版本化的
> [机器协议 schema](docs/reference/machine-protocol.schema.json)
> 才是实现权威。

AI 编程宿主速度很快，但可能偏离目标、在跨会话时丢失上下文，或把 Issue
当作无需审视的批准凭据。Slipway 将一个工作单元转化为受控、可恢复的 Run：宿主先调查
仓库，澄清真正需要人类决定的问题，实施有界变更，依据观察到的 diff 进行 Review，
最后汇总结果——整个过程始终由用户显式掌控。

Slipway 不是托管服务、项目跟踪器，也不取代你的 AI 编程工具。它是一个控制平面，
让 agent 的工作保持有界、可恢复，且始终如实可查。它不持有模型 provider 或 GitHub token；
由可信宿主获取来源，再由 CLI 进行验证。

```text
Objective Issue（可选的规划父级；永不可执行）
  └─ 自包含 Change Issue（唯一由 Issue 支撑的来源）
       └─ Run（一次固定 revision、可中断的尝试）
            orient → 必要时 clarify → implement → 基于观察到的 diff review → summarize
```

## 为什么选择 Slipway？

| 能力 | 带来的改变 |
| --- | --- |
| **Requirements-only** | Slipway 不维护 Spec、Delta 或永久 Requirements 注册表。开放的 Issue 是临时交付契约；交付后，代码和测试成为当前事实。 |
| **Issue-first，不是 Issue-gated** | 非微小工作从自包含 Change Issue 开始，但 GitHub 绝不能阻塞 Run。微小、敏感、紧急、离线或有意不跟踪的工作以 ad-hoc 方式开始。 |
| **固定来源** | Run 绝不信任可变的 `#42`。CLI 确定性解析严格 manifest，按 digest 固定每个 chapter，并且只存储有界 catalog 与采用 domain separation 的 revision。 |
| **六个显式能力** | 十个适配器仅生成 `run`、`clarify`、`propose`、`decompose`、`implement` 和只读的 `review`。没有 ambient hook、全局 router 或隐式调用。 |
| **七个公开命令** | `install`、`uninstall`、`list`、`doctor`、`run`、`status`、`stop`，另有版本化的隐藏 `_machine` 操作。机器协议是一份稳定契约。 |
| **如实恢复** | `.git/slipway/runs/` 下的追加式 journal 是恢复权威。Run 可以停止、恢复和重放；`ended` 只表示队列已空。 |
| **不作完成认证** | 测试失败、未运行测试、Review finding、脏工作树和 Issue 状态都不会阻塞推进。Slipway 只报告事实，不认证“完成”、可部署或可发布。 |
| **Issue 内容不可信** | Issue 正文、评论和 label 都是数据，而非指令。Issue 中的 prompt injection 和 credential 请求不具备宿主权限。 |
| **精确的破坏性权限** | 破坏性工作需要一次性、限定 scope 的结构化授权。自然语言中的“是”绝不构成授权；可信宿主只能提供 attestation，而不能构成人类在场的密码学证明。 |

## 设计理念

Slipway 遵循三条约束性规则：

- **用户拥有过程。** Slipway 只会因显式调用而启动。用户可以 skip、stop、resume 或
  接管任意 Action，无需说明理由。普通实现不会反复请求授权；真正需要人类决定的事项、
  source amendment、环境故障和破坏性工作才会暂停。
- **先查事实，再提问题。** 宿主在提问前先调查仓库、Git 状态和项目约定。
  能从代码中解决的决定绝不转交用户；真正需要人类决定的问题每次只问一个，
  并附上建议、理由和备选方案。
- **如实报告。** Slipway 报告观察到的变更、实际执行的活动、exit code、finding、
  已知问题和不确定性。它绝不会声称某项未执行的活动已经运行，也绝不会把空队列
  包装成正确性、交付状态或发布就绪认证。

Requirements 是临时交付契约，而不是系统的永久模型。只有当一个成果必然需要多个
独立 Change 时才创建 Objective；每个 Change 都必须自包含，不从父级或普通讨论评论中
继承运行时 Requirements。正文中第一个精确 marker 是 Level 权威；label、标题、
`ready-for-agent`、Project 字段、测试和 finding 都只是 warning-only projection，
绝不会阻塞 marker-valid Run。

请阅读[产品权威](docs/zh/reference/product-overview.md)、
[Issue 工作流](docs/zh/reference/issue-workflow.md)和
[架构](docs/zh/explanation/architecture.md)，了解完整模型。

## Quick Start

从由官方发布版本支持的渠道安装 Slipway，然后为你实际使用的 AI 工具生成宿主适配器。

| 平台 | 推荐方式 |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | 使用[安装指南](docs/zh/installation.md#linux-软件包)中的 `.deb`、`.rpm`、`.apk`、`tar.gz`、AUR 或容器镜像方式。 |
| Go 备用方式 | `go install github.com/signalridge/slipway@latest` |

```bash
slipway --version
cd your-repository
slipway install --tool claude
```

支持的工具 ID 为 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、
`opencode`、`pi`、`qwen` 和 `windsurf`。可以重复使用 `--tool`、传入逗号分隔的值，
或使用 `--tool all`。Kiro 首次安装时需要 `--surface ide|cli`。

### Ad-hoc 例外通道

对于微小、敏感、紧急、离线的工作，或你明确不希望创建 Issue 的情形，无需任何来源即可启动：

```bash
slipway run --budget 8 --json --root "$PWD" -- "add a CSV export to reports"
```

### 绑定 Issue 的 Run

可信宿主只获取一次以严格 manifest 寻址的 Source Bundle，并将临时 raw envelope 传给 CLI：

```bash
slipway run --budget 8 --json --root "$PWD" \
  --source-file /safe/temp/change-envelope.json -- "implement the bounded Change"
```

CLI 验证 Issue 正文中的 manifest 及其精确引用的评论，按 digest 固定每个 chapter，
并且只存储有界 catalog。本地读取 material 或恢复 Run 时，均不再需要该临时文件或 GitHub。

这就是完整的交互模型。在 AI 工具会话中，你显式调用某个 `slipway-<name>` 能力；
宿主每次执行一个 Action，仅在真正需要决定、source amendment、环境故障或破坏性确认时
暂停。用户行使控制权无需说明理由：“跳过这个”、“停止”、“接管”和“先做 X”都会按字面执行。

<details>
<summary><strong>CLI 会持有什么，又不会持有什么</strong></summary>

CLI **不会**：

- 持有 GitHub token 或调用模型 provider；
- 实现 GitHub/Project provider 或 tracker runtime；
- 扫描普通讨论评论，或把评论顺序视为权威；
- 创建、切换、绑定或删除 worktree；
- 声称 Issue 创建或正文 compare-and-swap 具备 exactly-once 语义；
- 承诺 journal 中绝无 secret。

CLI **会**：

- 严格验证经宿主 attestation 的 raw envelope，并拒绝未知字段、重复 JSON key、无效 UTF-8、
  BOM 和尾随数据；
- 确定性解析 manifest，并通过带 domain separation 的 digest 将 chapter 固定到私有的
  content-addressed material store；
- 每次返回一个有界、版本化的 Action，并附带结构化本地 reader；
- 维护追加式 journal 作为恢复权威，同时维护一份可替换的 projection；
- 暴露七个公开命令以及版本化的隐藏 `_machine` 操作。

</details>

## 工作原理

| 阶段 | Slipway 的要求 |
| --- | --- |
| `orient` | 提问之前先调查仓库事实、Git 状态和项目约定。建议下一 Action，或因真正需要的决定而暂停。 |
| `clarify` | 每次只处理一个有前后依赖的人类决定，并附上建议和取舍。无状态：不写文件、不创建 Issue，用户要求收尾时立即停止。面对完整请求时不提任何问题。 |
| `implement` | 执行当前 Action 授权的有界变更。如实报告命令、exit code、变更文件、已知问题和不确定性。 |
| `review` | 只读检查 Intent（是否满足固定的 Requirements？）与 Quality。绝不编辑代码、绝不返回 `needs_input`、绝不开启修复循环。 |
| `summarize` | 汇总 finding 和执行活动。验收后，Run 进入 `ended`。 |

Run 每次推进一个版本化 Action。路由以 diff 为先：当 CLI 观察到相对于不可变的 Run 起始
Git fingerprint 存在变化，且 Review 已启用时，无论宿主如何报告，都会路由到 Review。
Review 始终路由到 Summary，Summary 再路由到 `ended`。失败的活动和 Review finding
都是**数据**，绝不是 gate；它们进入 Summary，但不会触发自动修复循环。

## 六个能力，七个命令

```text
适配器生成：           run  clarify  propose  decompose  implement  review
公开 CLI 命令：        install  uninstall  list  doctor  run  status  stop
```

每个能力都必须显式调用。Clarify 遵循注明来源的
[Matt Pocock `grill-me` / `grilling`](https://github.com/mattpocock/skills)
方法：先调查事实，按依赖顺序每次处理一个决定并给出建议，在共同理解发生变化时予以确认，
始终保持无状态，并在用户要求收尾时立即停止。Review 只读，绝不修复，也不开启
re-review 循环。

隐藏的版本化 `_machine submit/answer/skip/resume/material` 操作为软自动驾驶循环提供支持，
详见[机器协议](docs/zh/reference/machine-protocol.md)。

<details>
<summary><strong>AI 工具适配器</strong></summary>

使用 `slipway install --tool <id>` 生成宿主工具载体，并使用
`slipway install --refresh` 刷新托管文件。生成的文件会跟踪 ownership，
因此刷新只会替换归 Slipway 所有的文件，而不会删除相邻的用户自定义内容。

| 工具 | 原生载体 | 显式调用方式 |
| --- | --- | --- |
| `claude` | `.claude/skills` | 调用 `slipway-<name>` skill |
| `codex` | `.codex/skills`（每个 skill 含 `agents/openai.yaml`） | `$slipway-<name>` |
| `copilot` | `.github/copilot/agents/*.agent.md` | 选择 `slipway-<name>` 自定义 agent |
| `cursor` | `.cursor/skills` | 调用 `slipway-<name>` skill |
| `kilo` | `.kilo/commands/*.md` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/*.md` | 手动包含 `#slipway-<name>` |
| `kiro` CLI | `.kiro/agents/*.json` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/*.md` | `/slipway-<name>` |
| `pi` | `.pi/skills` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills` | 调用 `slipway-<name>` skill |
| `windsurf` | `.windsurf/workflows/*.md` | `/slipway-<name>` |

适配器不会安装 ambient session hook、prompt-submit hook、launcher、全局 router 或独立的
技术验证能力。宿主设置不属于适配器 ownership，且永远不会被修改。

请参阅[宿主适配器](docs/zh/reference/adapters.md)和
[安装指南](docs/zh/installation.md)，了解精确载体、ownership 规则以及 Kiro `--surface` 的处理方式。

</details>

## Slipway 与其他工具的对比

大多数 AI 工作流系统使用 spec 文件和阶段提示词来组织工作。Slipway 的选择更聚焦于
**有界且诚实的权威模型**：生命周期状态保存在确定性 CLI 中，由 CLI 根据仓库重新计算事实，
并按 digest 固定来源，而不是相信 agent 的摘要。

<details>
<summary><strong>相邻工具及其取舍</strong></summary>

| 工具 | 模型 | 完成约束 |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | Spec 文件 + 斜杠命令 | 建议性检查清单与阶段提示词。 |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | Spec 驱动工作流 | 灵活的 spec 工作流；验证可选。 |
| [GSD Core](https://github.com/open-gsd/gsd-core) | Runtime 载体 + 阶段命令 | 强阶段循环；最终证明由 artifact 间接承载。 |
| [superpowers](https://github.com/obra/superpowers) | 自动触发的 skill | 强 agent 纪律；规则存在于模型上下文中。 |
| **Slipway** | 显式能力 + 固定来源 | 有界、可恢复的 Run；如实报告；无完成 gate。 |

Slipway 牺牲广度，换取诚实和控制。对于单行修改，它比完整 spec 框架更轻量
（直接使用 ad-hoc 即可）；而当你需要一条可恢复、来源固定、以 diff 观测为依据的轨迹，
并要求系统绝不静默重写历史或草率地为 Issue 背书时，它又严格得多。

</details>

## 恢复、隐私与证据

恢复权威位于 `.git/slipway/runs/<run-id>/`：

```text
.git/slipway/runs/<run-id>/
├── journal.jsonl   追加式状态转换权威
├── run.json        可替换的 projection
├── run.lock        串行化 journal mutation
└── materials/      content-addressed chapter blob（0600）
```

Journal 可能包含已接受的 Requirements、目标、用户回答和如实的命令摘要。Slipway
会尽量减少数据并对识别到的 credential 脱敏，但**不承诺 journal 中绝无 secret**——
应将 Run 目录视为本地私有数据。Unix mode 和 Windows current-user ACL 的设计意图
仍受 root、备份、malware、继承 ACL 以及同账户进程等因素限制。

删除 Run 目录只会移除恢复能力，并不等同于 secure erase、清除备份或销毁密钥。
请阅读[Run 与隐私](docs/zh/explanation/runs-and-privacy.md)、
[Windows 行为](docs/zh/reference/windows-rendering-and-durability.md)，以及如实记录的
[验收证据矩阵](docs/zh/reference/acceptance-evidence.md)。

## 文档

文档按任务组织：

- [从这里开始](docs/zh/start-here.md)——从安装到执行一次 Run 的最短路径。
- [产品权威](docs/zh/reference/product-overview.md)——四轴模型、六个能力和七个命令。
- [Issue 工作流](docs/zh/reference/issue-workflow.md)——Objective/Change marker、label、
  自包含性、GitHub 限制和发布对账。
- [安装指南](docs/zh/installation.md)——各平台安装方式和适配器命令。
- [命令](docs/zh/reference/commands.md)——公开命令和 JSON 接口。
- [机器协议](docs/zh/reference/machine-protocol.md)——版本化 Action / Outcome 契约和隐藏操作。
- [宿主适配器](docs/zh/reference/adapters.md)——十个宿主、六个能力和 ownership 安全规则。
- [架构](docs/zh/explanation/architecture.md)——package 布局和依赖方向。
- [Run 与隐私](docs/zh/explanation/runs-and-privacy.md)——journal 内容、保留策略和隐私承诺。
- [Windows 渲染与持久性](docs/zh/reference/windows-rendering-and-durability.md)
  ——argv 渲染和崩溃持久性。
- [验收证据](docs/zh/reference/acceptance-evidence.md)——证据类型和 35 个场景组成的矩阵。

## 验证

开发时可使用以下本地检查：

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

CI 会运行 Markdown、YAML 和 GitHub Actions lint、Go lint、Slipway testlint、跨平台 Go 测试、
race 测试、构建检查、原生 Windows cmd/PowerShell 套件、适配器 shell 验收和文档构建。

## 贡献

贡献采用 fork + pull request 工作流。请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)
了解贡献流程，并参阅[开发参考](docs/zh/contributing.md)了解开发细节。

## 许可证

Slipway 依据 [BSD 3-Clause License](LICENSE) 分发。
