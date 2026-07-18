# 宿主适配器

`slipway install` 会为七项明确的能力生成宿主原生入口：

```text
slipway-run  slipway-clarify  slipway-propose  slipway-decompose
slipway-implement  slipway-review  slipway-workflow
```

`run` 驱动可恢复 Run；`clarify` 是独立、无状态的决策对话；`propose` 与 `decompose` 准备 GitHub work item；`implement` 执行技术工作；`review` 只读；`workflow` 自主地把粗略想法编排为可发布的 Change 或 Objective 草稿，然后停在明确的 `propose` 与 `run` 边界。

![Slipway 宿主适配器：install 为十个受支持宿主写入原生入口；每个入口暴露相同的七项明确能力，只有 Run-backed 路径会与本地 CLI 交换版本化 JSON。](../../assets/diagrams/tool-adapters.svg)

## 生成目标

下表描述生成文件和预期调用方式。外部宿主的实际行为取决于其版本；仓库测试验证生成结果和协议文本，不等于对每个宿主 UI 进行 E2E 验证。

| ID | 生成目标 | 预期调用 |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | 调用 `slipway-<name>` skill。 |
| `codex` | `.codex/skills/slipway-*/SKILL.md`，每个 skill 带 `agents/openai.yaml` | `$slipway-<name>` |
| `copilot` | `.github/agents/slipway-<name>.agent.md` | 选择 custom agent。 |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | 调用 `slipway-<name>` skill。 |
| `kilo` | `.kilo/commands/slipway-<name>.md` 与 `.kilocode/slipway/capabilities/` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/slipway-<name>.md` 与 `.kiro/slipway/capabilities/` | 手工加入 `#slipway-<name>`。 |
| `kiro` CLI | `.kiro/agents/slipway-<name>.json` 与 `.kiro/slipway/capabilities/` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/slipway-<name>.md` 与 `.opencode/slipway/capabilities/` | `/slipway-<name>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | 调用 `slipway-<name>` skill。 |
| `windsurf` | `.windsurf/workflows/slipway-<name>.md` 与 `.windsurf/slipway/capabilities/` | `/slipway-<name>` |

Copilot agent 是 self-contained 文件。Kilo、Kiro、OpenCode 和 Windsurf 使用 thin native entry 指向 generated capability body；skill-native host 则将 capability body 放在 `SKILL.md` 中。

Kiro 首次安装需要 `--surface ide` 或 `--surface cli`。该选择会被记录，普通 refresh 不会静默切换。

## 主动调用

Adapter 不会安装 session-start hook、prompt-submit hook、launcher 或 global router，宿主设置也不属于适配器的所有权范围。能力由用户主动调用；在已经主动启动的 `slipway-run` 中，宿主可以继续推进范围明确的 Action loop，不必在每个普通步骤前重复索要授权。

Codex policy file 为每个能力禁用 implicit model invocation。其他目标使用各自 native explicit-entry surface 和共享指令。

## CLI 与宿主职责

CLI 负责：

- 验证并记录 Run；
- 选择下一个 Action；
- 观察 Git 与 workspace identity；
- 验证 source envelope 与 Outcome；
- 返回结构化恢复。

宿主负责：

- 读取仓库并执行技术工作；
- 调用模型；
- 在用户要求 issue-backed 工作时使用 GitHub 凭据；
- 构造临时 source envelope；
- 遵循 publication preview、confirmation 与 reconciliation 指令。

因此 `propose` 与 `decompose` 描述宿主应如何使用 GitHub API；Go CLI 并不提供 GitHub publication transaction。详见 [GitHub Issue 工作流](../guides/github-issues.md)。

## 安装与刷新

```bash
slipway install --tool claude
slipway install --tool kiro --surface ide
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

首次 Kiro 与 `--tool all` 限制见[安装](../installation.md)。

## Ownership 安全

![Slipway 适配器安装与 ownership 安全：install 只写入 宿主本地 capability 文件，并把其路径与 SHA-256 记录进每个 host 的 ownership manifest；该 manifest 是后续修改受管文件的唯一授权来源；refresh 与 uninstall 会按 hash 把每个记录文件重新分类为 pristine、missing 或 modified，只修改 pristine 与 missing，对 modified、未知或不安全的内容一律保留或拒绝并报告原因。](../../assets/diagrams/install-ownership.svg)

每个 host root 下都有 Slipway ownership manifest，记录 repository-relative path 与 SHA-256。Refresh 和 uninstall 只修改仍与记录 hash 匹配的文件。

被用户修改的 capability、未知文件、modified sentinel、malformed manifest、path escape、duplicate claim 或 unsafe symlink 不会被静默纳入 managed content；操作会保留或拒绝并报告原因。Transaction recovery artifact 与普通 preserved user file 分开报告。

Generated sentinel 只表示 installation health，不代表 ownership。只有 manifest 可以授权后续 managed-file change；不支持的 manifest version 会在 mutation 前失败。
