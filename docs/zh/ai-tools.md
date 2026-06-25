# AI 工具适配器

`slipway init --tools` 会导出宿主工具文件，让 AI 编码工具能够调用 Slipway 命令，并从当前项目加载受治理的 skill 指令。

<div align="center" markdown>

![Slipway 工具适配器：slipway init --tools 为 Claude、Codex、Copilot、Cursor、Kilo、Kiro、OpenCode、Pi、Qwen 和 Windsurf 生成各工具的适配器包，并附带 .slipway.yaml 运行时配置；每个适配器生成的 skills 和命令都会把受治理的操作路由到 slipway CLI](assets/diagrams/tool-adapters.svg)

</div>

## 支持的工具

| 工具 ID | Skills 路径 | 命令路径 | 调用方式 |
| --- | --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `.claude/commands/slipway/*.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `.codex/skills/slipway-*/SKILL.md` | `$slipway-<command>` (or `/skills`) |
| `copilot` | `.github/skills/slipway-*/SKILL.md` | `.github/prompts/slipway-<command>.prompt.md` | `/slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `.cursor/commands/*.md` | `/slipway-<command>` |
| `kilo` | `.kilocode/skills/slipway-*/SKILL.md` | `.kilocode/workflows/slipway-<command>.md` | `/slipway:<command>` |
| `kiro` | `.kiro/skills/slipway-*/SKILL.md` | `.kiro/skills/slipway-<command>/SKILL.md` | `@slipway:<command>` or host skill picker |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `.opencode/commands/slipway-*.md` | `/slipway-<command>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `.pi/prompts/slipway-<command>.md` | `/slipway-<command>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `.qwen/skills/slipway-<command>/SKILL.md` | `/slipway-<command>` or host skill picker |
| `windsurf` | `.windsurf/skills/slipway-*/SKILL.md` | `.windsurf/workflows/slipway-<command>.md` | `/slipway-<command>` |

Codex、Kiro 和 Qwen 的命令会作为可发现的逐命令 skill 生成在各适配器的 skills 目录下。基于 prompt 和基于 workflow 的宿主则改为生成命令文件。所有生成的命令界面都会调用 `slipway` CLI；宿主文件本身不实现独立的生命周期、评审或证据逻辑。Slipway 不再写入全局的 Codex prompt 文件，Codex 刷新也不会清理宿主全局的 prompt 目录。基于 prompt 的项目级适配器在刷新时仍会移除 Slipway 所拥有的、已废弃的 prompt 文件。

## 生成适配器

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools all
slipway init --tools none   # initialize runtime layout only, no adapter files
```

刷新受管文件，并清理 Slipway 所拥有的、已废弃的适配器产物：

```bash
slipway init --tools all --refresh
```

刷新自动检测到的受管适配器：

```bash
slipway init --refresh
```

Slipway 通过它生成的哨兵文件来检测适配器，而不是仅凭一个裸的 `.claude`、`.codex`、`.cursor`、`.opencode`、`.pi`、`.qwen`、`.kiro`、`.windsurf` 或 `.kilocode` 目录。所有权清单会在刷新时保护生成的文件；只有哨兵、缺少清单的遗留适配器可以被纳入清单跟踪，而缺少哨兵的路径冲突则保持 fail-closed，除非现有内容已经与生成的输出一致。Copilot 把这部分受管状态保存在 `.github/copilot/slipway` 下，而不是把整个共享的 `.github` 目录树都当作适配器所有。刷新会移除 Slipway 所拥有的遗留 shell hook 启动脚本和已废弃的 `bash "<hook>.sh"` hook 配置项，同时保留生成文件旁边用户自有的 hook、prompt、workflow 和 skill。

## 生成的命令界面

那些选择生成宿主 prompt 的命令，会在每个工具上提供一套命令界面：

- 在 Claude、Copilot、Cursor、Kilo、OpenCode、Pi 和 Windsurf 上是 prompt 和 workflow 文件。
- 在 Codex、Kiro 和 Qwen 上是逐命令 skill。

像 `slipway tool` 这样仅限 CLI 的辅助命名空间，在 Slipway 二进制中仍是公开的,但不会生成宿主命令包装;生成的 skill 会直接调用 `slipway tool ...` 子命令。

生成的 hook 除了 `slipway` 二进制外没有其他依赖。手动辅助命令可以使用显式的、已认证的后端或领域工具:GitHub 相关辅助命令优先使用 `gh`,在 `gh` 不可用或报告需要认证的错误时回退到 token API,两种后端都不存在时则 fail closed。

核心生命周期命令:

- `new` (`$slipway-new`)
- `intake` (`$slipway-intake`)
- `plan` (`$slipway-plan`)
- `implement` (`$slipway-implement`)
- `review` (`$slipway-review`)
- `fix` (`$slipway-fix`)
- `done` (`$slipway-done`)
- `next` (`$slipway-next`)
- `run` (`$slipway-run`)
- `status` (`$slipway-status`)

发现类命令:

- `codebase-map` (`$slipway-codebase-map`)

情境类命令:

- `preset` (`$slipway-preset`)
- `validate` (`$slipway-validate`)
- `abort` (`$slipway-abort`)
- `cancel` (`$slipway-cancel`)
- `delete` (`$slipway-delete`)
- `repair` (`$slipway-repair`)
- `evidence` (`$slipway-evidence`; the wave-orchestration host records task evidence via `slipway evidence task ...`)

辅助命令:

- `tool` 仅限 CLI。没有 `$slipway-tool`,也没有生成的宿主 prompt 包装;生成的 skill 会直接调用 `slipway tool <helper>`。

诊断类命令:

- `health` (`$slipway-health`)
- `instructions` (`$slipway-instructions`)

安装类命令:

- `init` (`$slipway-init`)

workflow skill 的命令参考索引了所有生成的命令界面。对于仅限 CLI 的辅助命令,请使用生成的 skill 指令中指明的显式 `slipway tool ...` 命令。

## 界面清单

`docs/SURFACE-MANIFEST.json` 是已提交的清单,记录了生成的适配器、命令、skill、JSON 和文档界面。它由 Slipway 所拥有的 Go 权威源重新生成,不应手工编辑:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

在新增一个生成的工具、命令、skill、JSON 契约或文档界面后运行 `--write`,然后在清单行所指定的文件中保留匹配的文档标记。

## OpenCode 注意事项

OpenCode 把项目命令存储为 `.opencode/commands/` 下的 Markdown 文件。Slipway 会在以下目录生成扁平的 OpenCode 命令文件:

```text
.opencode/commands/
```

命令文件名会成为 OpenCode 的命令 ID。例如:

```text
.opencode/commands/slipway-new.md
```

调用方式为:

```text
/slipway-new
```

某些 OpenCode 版本会在命令选择器中以项目前缀显示项目命令。生成的文件路径始终是稳定的 Slipway 契约。

生成的 OpenCode skill 位于:

```text
.opencode/skills/
```

由于 OpenCode 没有 `settings.json`,那个建议性的会话 hook 会以平台原生的启动脚本形式生成:

```text
.opencode/hooks/slipway-session-start
.opencode/hooks/slipway-session-start.ps1
.opencode/hooks/slipway-session-start.cmd
```

Cursor 也采用同样的模式,生成 `.cursor/hooks/slipway-session-start` 以及配套的 `.ps1` 和 `.cmd` 文件。这些启动脚本只负责委托给 `slipway hook ...`;hook 的实际行为存在于 Slipway 二进制中。

## 其他适配器注意事项

Copilot 把命令 prompt 存储在 `.github/prompts/` 下,使用 `.prompt.md` 扩展名,生成的 skill 存放在 `.github/skills/`。它的哨兵和所有权清单位于 `.github/copilot/slipway` 下。

Pi 把命令 prompt 存储在 `.pi/prompts/`,生成的 skill 存放在 `.pi/skills/`。Slipway 还会合并 `.pi/settings.json`,使 `enableSkillCommands` 为 `true`、`./skills` 列入 `skills`、`./prompts` 列入 `prompts`。

Qwen 和 Kiro 把命令以生成的命令 skill 形式暴露,而不是单独的 prompt 文件。Qwen 会为会话启动 hook 写入 `.qwen/settings.json`。Kiro 的命令 skill 使用 `@slipway:<command>`。

Windsurf 和 Kilo 把命令以 workflow 文件的形式暴露,分别位于 `.windsurf/workflows/` 和 `.kilocode/workflows/`。Kilo 使用 `/slipway:<command>` 触发器,即便它的 workflow 文件命名为 `slipway-<command>.md`。

## 支持 settings 的宿主

Claude (`.claude/settings.json`) 和 Qwen (`.qwen/settings.json`) 在各自的 settings 文件中内联注册 hook,而不是通过启动脚本。Slipway 会把裸的 `slipway hook ...` 命令直接写入 `settings.json`:

- `PostToolUse` 上的 `slipway hook context-pressure`
- `SessionStart` 上的 `slipway hook session-start`

Claude 会注册这两个 hook。Qwen 只注册 `SessionStart`。这些通过 settings 注册的 hook 不会生成启动脚本文件;命令会在 `PATH` 上解析 `slipway` 二进制,hook 行为存在于该二进制中。Pi 的 settings 只用于注册 skill 和 prompt,不涉及 hook 设置。

## 安全规则

- 除非你有意定制本地宿主行为,否则不要编辑生成的 Slipway 适配器文件。
- 在 Slipway 变更后,使用 `slipway init --refresh` 更新生成的文件,并清理 Slipway 所拥有的、已废弃的适配器条目。
- 保留相邻 AI 工具目录中用户自有的文件。
- 当仓库应当为所有贡献者初始化时,提交 `.slipway.yaml`;在提交生成的适配器文件之前,按仓库策略对它们进行评审。
