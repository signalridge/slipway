# AI 工具适配器

本页是生成式 AI 工具适配器的 Diataxis 参考入口。更详细的旧版适配器参考仍可在
[AI 工具适配器](../ai-tools.md) 查阅。

`slipway init --tools` 会导出宿主文件，让 AI 编码工具在当前项目中发现 Slipway
命令和受治理的 skill 指令。这些文件最终都路由回 CLI，本身并不是独立的治理引擎。

## 支持的工具 ID

| 工具 ID | 生成的 skill 路径 | 调用方式 |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `$slipway-<command>` 或 `/skills` |
| `copilot` | `.github/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `kilo` | `.kilocode/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `kiro` | `.kiro/skills/slipway-*/SKILL.md` | `@slipway:<command>` 或宿主 skill 选择器 |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `/slipway-<command>` 或宿主 skill 选择器 |
| `windsurf` | `.windsurf/skills/slipway-*/SKILL.md` | `/slipway-<command>` |

## 生成或刷新

```bash
slipway init --tools codex
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --refresh
slipway init --tools all --refresh
```

刷新会识别 Slipway 生成的标记，同时保留生成的适配器目录旁那些用户自有的文件。

## 生成的命令界面

选择接入宿主提示词的命令会生成命令界面。Codex、Kiro 和 Qwen 将它们暴露为命令
skill，其他宿主则暴露为提示词、命令或工作流文件：

- Claude：`.claude/commands/slipway/<id>.md`
- Copilot：`.github/prompts/slipway-<id>.prompt.md`
- Cursor：`.cursor/commands/slipway-<id>.md`
- Kilo：`.kilocode/workflows/slipway-<id>.md`
- OpenCode：`.opencode/commands/slipway-<id>.md`
- Pi：`.pi/prompts/slipway-<id>.md`
- Windsurf：`.windsurf/workflows/slipway-<id>.md`

生成的适配器文件都路由到 `slipway` CLI。宿主界面不会实现独立的生命周期、评审或
证据引擎。

Codex 命令 skill 的调用标记：

```text
$slipway-new
$slipway-intake
$slipway-plan
$slipway-implement
$slipway-review
$slipway-fix
$slipway-done
$slipway-next
$slipway-run
$slipway-status
$slipway-codebase-map
$slipway-handoff
$slipway-preset
$slipway-validate
$slipway-abort
$slipway-cancel
$slipway-delete
$slipway-repair
$slipway-evidence
$slipway-health
$slipway-instructions
$slipway-init
```

`slipway tool` 仅限 CLI 使用。它没有生成的宿主命令包装，生成的 skill 在需要时直接
调用相应的辅助子命令。

## 设置与归属

支持设置合并的适配器会直接合并宿主设置，而不是要求 agent 手动编辑生成的文件：

- Claude 注册简洁的内联 `slipway hook ...` 设置命令。
- Pi 写入 `.pi/settings.json`，设置 `enableSkillCommands=true` 并注册 `./skills`
  和 `./prompts`。
- Qwen 写入 `.qwen/settings.json` 以注册会话启动钩子。
- Codex 写入 `.codex/config.toml`，为 `SessionStart` 和 `UserPromptSubmit` 注册
  钩子；在用户信任该仓库及每个钩子之前，这些钩子处于不生效状态，且 Slipway 绝不
  会修改全局的 Codex 信任设置。

每个适配器都由 Slipway 生成的哨兵文件和归属清单跟踪，存放在适配器根目录下的
`slipway/` 目录中。Copilot 将这份受管状态存放在 `.github/copilot/slipway`，这样刷新
时就不会把 `.github` 的其余部分当作 Slipway 所有。

## 安全规则

- 以当前 worktree 的 CLI 输出为权威。
- 命令、skill 或钩子契约变更后，刷新生成的适配器。
- 保留相邻 AI 工具目录中用户自有的文件。
- 当项目默认值需要共享时，提交 `.slipway.yaml`。
- 提交生成的适配器文件前，按仓库策略对其进行评审。

## 完整细节

关于生成的钩子细节、OpenCode 说明、支持设置合并的宿主以及旧版清理行为，参见
[AI 工具适配器](../ai-tools.md)。
