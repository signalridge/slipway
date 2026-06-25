[English](README.md) · **简体中文** · [日本語](README.ja.md)

<div align="center">

<img alt="Slipway - Governance CLI for AI-assisted software delivery" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=materialformkdocs&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[文档](https://signalridge.github.io/slipway/) |
[从这里开始](docs/start-here.md) |
[快速上手](#快速上手) |
[安装](docs/installation.md) |
[发布说明](CHANGELOG.md)

<br/>

[English](README.md) · **简体中文** · [日本語](README.ja.md)

</div>

# Slipway

**面向 AI 辅助软件交付的本地、Git 原生治理 CLI。你的 agent 负责写代码，Slipway 负责判定这个改动是否真的完成了。**

AI 编码 agent 速度很快，但它们可能跳过验证、偏离计划，或者在当前 worktree 还没有给出证据之前就上报“完成”。Slipway 把一个工作单元变成一项受治理的改动，带有生命周期状态、规划工件、任务证据、评审证据，以及一份留在仓库里的最终归档。

Slipway 不是托管服务，不是项目管理工具，也不是 AI 编码工具的替代品。它是让 agent 的工作可被审视、且做到 fail-closed（失败即拦截）的控制平面。

它的核心价值不是又一份检查清单。Slipway 把当前 worktree、生成的宿主指令、生命周期状态和评审上下文彼此隔开，再让 CLI 重新计算这些部分是否仍然一致。

## 为什么选 Slipway？

| 能力 | 它带来的改变 |
| --- | --- |
| **可编译的完成闸门** | `slipway done` 在归档前重新核查当前的评审、验证、范围和护栏证据。证据缺失或过期都会阻止收尾。 |
| **轻量 AI 适配器** | 生成的宿主适配文件（Claude、Codex、Cursor、OpenCode、Copilot、Kilo、Kiro、Pi、Qwen、Windsurf）把 agent 引导回 CLI，而不是各自变成独立的工作流引擎。 |
| **大白话入口** | 执行 `slipway init --tools <id>` 后，用户可以用日常语言描述改动；生成的入口 skill 会把 agent 引导进受治理的生命周期。 |
| **以当前 worktree 为准** | `status`、`validate` 和 `next` 都从持有改动的 worktree 重新计算状态，而不是信任过期的摘要或归档记录。 |
| **上下文隔离核查** | 计划审计、实现、选定的 S3 评审同行、修复，以及终末的 `ship-verification` 闸门，各自携带不同的 context-origin 证据并接受顺序核查。 |
| **绑定 worktree 的执行** | 偏重探查的改动可以在专用的 `.worktrees/<branch>` 检出中运行；执行继续前会校验 worktree 路径和分支绑定。 |
| **实际改动的波次审计** | 按依赖排序的波次可以并行运行，实现完成后 Slipway 会审计真实变更的文件、执行者句柄、派发模式和范围约束。 |
| **仓库自持的审计轨迹** | `artifacts/changes/`、`.git/slipway/runtime/`、生命周期事件和归档 bundle，让记录在会话结束后仍可查阅。 |

## 快速上手

安装 Slipway，初始化仓库，并为你实际使用的 AI 工具生成适配器：

```bash
brew install --cask signalridge/tap/slipway
# or
go install github.com/signalridge/slipway@latest

slipway init --tools codex
```

其它适配器 ID 包括 `claude`、`codex`、`cursor`、`opencode`、`copilot`、`kilo`、`kiro`、`pi`、`qwen`、`windsurf`、`all` 和 `none`。

这一步一次性配置就是全部的安装工作。之后你不必手动操作 Slipway。在你的 AI 工具会话里，用日常语言描述改动即可：

> 给 export 命令加一个 `--dry-run` 模式。

`slipway init` 生成的适配器会把这个请求引导进受治理的生命周期。入口 skill 接手这项改动，agent 会替你跑完 `slipway` 的 intake、规划、实现、评审和完成闸门。只有当 Slipway 返回需要你决策的 skill 交接、检查点、阻塞项或就绪待完成（done-ready）状态时，它才会停下来。

你只管用大白话，Slipway 始终掌握“改动是否真的完成”的判定权。你无需背命令序列，也无需在脑子里维护生命周期状态——那是 agent 的活儿，背后由 CLI 兜底。只读视图随时供你查看 agent 所看到的内容：

```bash
slipway status --json
slipway next --json --diagnostics
```

要在多会话之间保持连贯，使用由命令自持的咨询性交接：

```bash
slipway handoff write
slipway handoff show --brief
slipway handoff show
```

`slipway handoff write` 会刷新每项改动的运行时交接骨架和机器可读的头部。这个头部只携带身份和新鲜度字段，不会快照生命周期状态或下一步动作。新会话仍然要跑 `slipway status --json` 和 `slipway next --json` 来获取权威状态。

<details>
<summary><strong>命令优先的生命周期</strong></summary>

想自己驱动生命周期，或在 CI 中编写脚本？下面这些就是 agent 替你运行的同一批命令，开放出来供你直接使用：

```bash
slipway new "refresh governance docs" --preset standard
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway validate --json
slipway done --json
```

`slipway run --json --diagnostics` 是快捷驱动器。它会委派给当前主阶段对应的命令，并在面向操作者的边界处停下。

### 执行自动模式

`.slipway.yaml` 中的 `execution.auto` **默认关闭**。一旦启用（或用 `slipway run --auto` 按次覆盖），`slipway run` 会在先前已授权的前提下自动跳过纯节奏性的暂停——评审批次、非敏感的 skill 交接，以及 **新鲜的** 人工核验检查点——而不再停下来要求重新确认。`slipway run --no-auto` 会强制单次运行回到手动节奏（`--no-auto=false` 不构成肯定式覆盖，会回落到配置）。

配置层面的 `execution.auto` 同样适用于各阶段命令（`slipway intake` / `slipway plan` / `slipway implement`），它们的自动推进行为与 `run` 一致，但不提供按阶段的标志；按次的 `--auto` / `--no-auto` 覆盖只存在于 `slipway run` 上。

自动模式绝不放松治理。`security-review` 边界、敏感/护栏确认、intake 的 Approved Summary、decision 和 human_action 检查点、过期或新鲜度未知的检查点,以及每一道证据闸门,都 **绝不** 自动推进；它们始终硬停，等待操作者明确输入和新鲜证据。仅用于升级的预设自动确认只会提高治理严格度（绝不降低），因此不属于上述这些红线。

</details>

## 工作原理

<div align="center">
  <img alt="Slipway governed lifecycle: new, S0 Intake, S1 Plan, S2 Implement, S3 Review, done-ready, done" src="docs/assets/diagrams/lifecycle.svg" width="920">
</div>

| 阶段 | Slipway 期望什么 |
| --- | --- |
| `S0_INTAKE` | 意图、范围、待解问题、风险等级和初始授权。 |
| `S1_PLAN` | 调研、需求、决策、任务计划和计划审计证据。 |
| `S2_IMPLEMENT` | 按依赖排序的波次、变更文件和任务证据。 |
| `S3_REVIEW` | 选定的同行评审、修复证据，以及终末的 `ship-verification` 闸门（一次权威的完整测试套件、验收证据、新鲜度复核、保障记录和评审者独立性证明）。 |
| `done` | 在 `artifacts/changes/archived/<slug>/` 下的终末归档。 |

`change.yaml` 掌握当前的生命周期权威。Markdown 工件解释这项工作，YAML 验证记录为特定阶段提供证据，生命周期事件给出只追加（append-only）的变更轨迹。只读视图（`status`、`validate`、`next`）是会话恢复或改动让人困惑时第一个该看的地方。主要的变更入口是 `slipway new`、`slipway intake`、`slipway plan`、`slipway implement`、`slipway review`、`slipway fix`、`slipway done`，以及 `slipway run` 快捷驱动器。

## 设计理念

Slipway 遵循三条项目准则：

- **唯一的当前权威。** `change.yaml` 掌握生命周期状态；日志和 Markdown 是辅助，永远不取代它。
- **隔离上下文，事后核查。** 编写、审计、评审、修复和 ship-verification 证据都作为各自独立的参与方记录在案；闸门负责核查这条独立性链条没有坍塌。
- **人可读、机可校验。** 人能评审这些工件，同时 CLI 从结构化输入重新推导新鲜度。
- **够用的最小控制平面。** 宿主适配器保持轻量；治理逻辑存在于 CLI 和仓库工件中。

更简短的说明见 [Design](docs/explanation/design.md) 和 [Workflow](docs/explanation/workflow.md)，或参阅 [Design Philosophy](docs/design.md) 和 [Governed Workflow](docs/workflow.md) 中的旧版深入解读。

<details>
<summary><strong>底层强制执行的几个维度</strong></summary>

在闸门背后，每个阶段都持有引擎重新推导、而非直接信任的证据。下面这些就是让伪造的“完成”露馅的实现维度：

| 维度 | 引擎行为 |
| --- | --- |
| 经证明的新鲜上下文 | 评审、计划审计、修复和收尾记录各自携带不同的 context-origin 证据并接受顺序核查。 |
| 防篡改的证据 | 新鲜度由变更文件、工件、run-summary 版本、终末 `ship-verification` 套件的运行,以及运行时任务证据推导而来,而不是来自某个写着 `pass` 的文件。 |
| 双向的并行安全 | 计划中文件互不相交的波次之后,会对真实变更的文件、执行者句柄、派发模式和范围约定进行审计。 |
| 范围约束 | `target_files` 和已声明的豁免会与真实 diff 对照核查；越界编辑会 fail-closed（失败即拦截）。 |
| 感知漂移的恢复 | 计划或证据出现漂移时,会就地重开改动,`slipway next` 会指明向前的修复命令。 |
| 本地优先的审计 | 活跃和归档记录都留在仓库里,运行时证据存放在 `.git/slipway/runtime/` 下。 |
| 按风险分级的护栏 | 敏感领域在批准上线前要求领域感知的评审、高风险检查和明确证据。 |

</details>

## Slipway 与同类工具对比

多数 AI 工作流系统擅长把工作组织起来。Slipway 的赌注更窄：强制执行——最终的生命周期权威落在一个确定性的 CLI 里，由它从仓库证据重新计算状态。

<details>
<summary><strong>相邻工具与取舍</strong></summary>

| 工具 | 你如何驱动它 | 完成的强制力 |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | `/speckit.*` 斜杠命令 | 咨询性检查清单和阶段提示词。 |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | `/opsx:*` 斜杠命令 | 灵活的规范工作流；验证是可选的。 |
| [spec-kitty](https://github.com/Priivacy-ai/spec-kitty) | `/spec-kitty.*` 命令外加 autopilot | 有一些状态闸门，但评审仍是咨询性的。 |
| [GSD Core](https://github.com/open-gsd/gsd-core) | 运行时视图外加 `/gsd-*` 阶段命令 | 阶段循环和新鲜上下文编排都很强；但最终证明仍以工作流工件为中介。 |
| [superpowers](https://github.com/obra/superpowers) | 自动触发的 skills | agent 纪律性强，但规则存在于模型上下文里。 |
| **Slipway** | 经轻量适配器用大白话驱动，或直接用 CLI | 可编译、fail-closed 的闸门，背后有仓库证据支撑。 |

Slipway 用广度换取权威。它支持的一等公民适配器比那些覆盖面广的提示词框架要少，但每个生成的接口都引导回同一个 CLI。在做完即弃的小编辑上，它比轻量提示词包更重；但当过期证据、范围漂移或高风险领域原本容易被漏掉时，它要严格得多。

</details>

## AI 工具适配器

用 `slipway init --tools <id>` 生成宿主工具接口，用 `slipway init --refresh` 刷新受管文件。生成的文件带有归属追踪，因此刷新时可以替换 Slipway 自持的文件，而不会删掉旁边用户自持的定制内容。

<details>
<summary><strong>各工具生成的接口</strong></summary>

| 工具 | 生成的接口 |
| --- | --- |
| Claude | `.claude/skills/slipway-*/SKILL.md`、`.claude/commands/slipway/*.md`、`.claude/settings.json` 钩子条目 |
| Codex | `.codex/skills/slipway-*/SKILL.md` 入口、命令和治理 skill；`.codex/config.toml` 中的 SessionStart 和 UserPromptSubmit 钩子条目 |
| Cursor | `.cursor/skills/slipway-*/SKILL.md`、`.cursor/commands/*.md`、会话启动钩子启动器 |
| OpenCode | `.opencode/skills/slipway-*/SKILL.md`、`.opencode/commands/slipway-*.md`、会话启动钩子启动器 |
| Copilot | `.github/skills/slipway-*/SKILL.md`、`.github/prompts/slipway-*.prompt.md`、`.github/copilot/slipway` 受管状态 |
| Kilo | `.kilocode/skills/slipway-*/SKILL.md`、`.kilocode/workflows/slipway-*.md` |
| Kiro | `.kiro/skills/slipway-*/SKILL.md` 入口、命令和治理 skill |
| Pi | `.pi/skills/slipway-*/SKILL.md`、`.pi/prompts/slipway-*.md`、`.pi/settings.json` skill/prompt 注册 |
| Qwen | `.qwen/skills/slipway-*/SKILL.md` 命令 skill、`.qwen/settings.json` 钩子条目 |
| Windsurf | `.windsurf/skills/slipway-*/SKILL.md`、`.windsurf/workflows/slipway-*.md` |

导出的生成 skill 行按公开的 skill 目录固定：`slipway/SKILL.md`、`slipway-ci-triage/SKILL.md`、`slipway-code-quality-review/SKILL.md`、`slipway-codebase-mapping/SKILL.md`、`slipway-coding-discipline/SKILL.md`、`slipway-context-assembly/SKILL.md`、`slipway-coverage-analysis/SKILL.md`、`slipway-git-recovery/SKILL.md`、`slipway-incident-response/SKILL.md`、`slipway-independent-review/SKILL.md`、`slipway-intake-clarification/SKILL.md`、`slipway-plan-audit/SKILL.md`、`slipway-research-orchestration/SKILL.md`、`slipway-root-cause-tracing/SKILL.md`、`slipway-security-review/SKILL.md`、`slipway-ship-verification/SKILL.md`、`slipway-spec-compliance-review/SKILL.md`、`slipway-spec-trace/SKILL.md`、`slipway-tdd-governance/SKILL.md`、`slipway-test-design/SKILL.md`、`slipway-wave-orchestration/SKILL.md`，以及 `slipway-worktree-preflight/SKILL.md`。

Codex 使用仓库本地的 `.codex/config.toml` 钩子，用于有界的 SessionStart 交接指针和以过期为条件的 UserPromptSubmit 写入提示。这些钩子在仓库以及每个钩子被用户信任之前都处于惰性状态；Slipway 绝不修改 Codex 的全局信任配置。

完整的命令和 skill 清单见 [AI Tool Adapters](docs/reference/ai-tools.md) 和生成的 [Surface Manifest](docs/SURFACE-MANIFEST.json)。

</details>

## 运行时文件

<details>
<summary><strong>Slipway 写入的仓库状态</strong></summary>

| 路径 | 角色 |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | 当前生命周期和路由权威。 |
| `artifacts/changes/<slug>/*.md` | 意图、调研、需求、决策、任务和保障记录。 |
| `artifacts/changes/<slug>/verification/` | 供上线闸门消费的 skill 验证记录。 |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | 只追加的生命周期变更轨迹。 |
| `.git/slipway/runtime/changes/<slug>/evidence/` | Git 本地的任务证据和运行时证明。 |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | 由命令自持、每项改动各自的咨询性续接笔记，由 `slipway handoff` 写入/读取；它绝不是生命周期权威、证据、新鲜度或闸门。 |
| `.git/slipway/locks/change-create.lock`、`.git/slipway/locks/repair.lock` | 用于改动创建和修复的工作区/范围级协调锁。它们刻意不是按改动划分的，因为它们保护的是在稳定 change slug 出现之前或之外就已开始的操作。 |
| `artifacts/changes/archived/<slug>/` | `slipway done` 之后的终末记录。 |
| `artifacts/codebase/` | 仓库范围的上下文映射，用于棕地（brownfield）规划和评审。 |
| `.worktrees/<branch>/` | 改动被隔离时使用的专用受治理 worktree。 |

AI 工具会话从项目根目录读取生成的宿主接口。受治理的 worktree 持有代码改动，但根目录的宿主适配文件不会被复制进每个 worktree。`.codex/config.toml` 中生成的 Codex 钩子在仓库被信任、且每个钩子被用户信任之前都处于惰性状态；Slipway 绝不修改 Codex 的全局信任配置。诸如 `.git/slipway/runtime/handoff.md` 这类旧版仓库级交接文件会被作为本地运行时卫生问题报告出来，不会被当作当前改动的权威。

</details>

## 文档

文档按任务组织：

- [Start Here](docs/start-here.md)：从安装到完成一项受治理改动的最短路径。
- [Real-World Scenarios](docs/real-world-scenarios.md)：落地采用模式。
- [First Governed Change](docs/tutorials/first-governed-change.md)：手把手教程。
- [Onboarding Existing Codebase](docs/tutorials/onboarding-existing-codebase.md)：棕地（brownfield）接入。
- [Install and Refresh Adapters](docs/how-to/install-and-refresh-adapters.md)：适配器的运维命令。
- [Recover and Troubleshoot](docs/how-to/recover-and-troubleshoot.md)：fail-closed 恢复。
- [Commands](docs/reference/commands.md)：命令和 JSON 接口参考。
- [AI Tool Adapters](docs/reference/ai-tools.md)：生成的宿主文件和调用方式。
- [Design](docs/explanation/design.md) 和 [Workflow](docs/explanation/workflow.md)：概念与设计动机。

## 验证

开发时有用的本地检查：

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go run ./internal/testlint/cmd/testlint ./...
golangci-lint run --timeout=5m
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
(cd website && npm run build)
```

CI 会运行 Markdown/YAML/action 的 lint、Go lint、Slipway testlint、跨平台的 Go 测试、竞态测试、内核覆盖率、构建检查、安全扫描、Nix 检查和文档工作流。

## 参与贡献

贡献走 fork-and-pull-request 流程。贡献流程见 [CONTRIBUTING.md](CONTRIBUTING.md)，开发细节见 [docs/contributing.md](docs/contributing.md)。

## 许可证

Slipway 基于 [BSD 3-Clause License](LICENSE) 授权。

## 仓库状态

![Repobeats analytics image](https://repobeats.axiom.co/api/embed/20e468225cab8a858d9bc969314a0e9c3d12bddb.svg "Repobeats analytics image")
