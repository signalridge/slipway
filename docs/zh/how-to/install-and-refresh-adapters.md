# 如何安装和刷新适配器

当你需要为当前仓库准备一个可用的 Slipway 二进制文件以及生成好的 AI 工具接口时，参照本指南操作。

完整的发布矩阵、校验和、容器镜像、包管理器渠道以及源码构建细节，参见 [安装](../installation.md)。

## 安装 CLI

尽量使用基于发布版本的渠道：

```bash
brew install --cask signalridge/tap/slipway
```

当没有现成的发布包，或你确实想要一个由 Go 管理的二进制文件时，使用 Go install：

```bash
go install github.com/signalridge/slipway@latest
```

验证安装：

```bash
slipway --help
```

如果某个 AI 工具在未知的 registry 中找到同名的包，先停下来确认归属再安装。

## 在仓库中初始化 Slipway

在仓库根目录执行：

```bash
slipway init --tools codex
```

常见的适配器选择：

```bash
slipway init --tools claude
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`--tools none` 只初始化运行时布局和 `.slipway.yaml`，不写入任何 host 适配器文件。

提交生成的文件前，先检查 diff：

```bash
git status --short
git diff -- .slipway.yaml .claude .codex .cursor .opencode
```

当希望整个仓库共享 Slipway 的默认配置时，提交 `.slipway.yaml`。生成的适配器文件是否提交，请按仓库自身的策略决定。

## 刷新已有适配器

刷新自动检测到的、由 Slipway 管理的适配器：

```bash
slipway init --refresh
```

刷新指定的一组适配器：

```bash
slipway init --tools codex,opencode --refresh
```

刷新所有受支持的适配器：

```bash
slipway init --tools all --refresh
```

刷新依据 Slipway 生成的标记来识别目标。它不会把一个空的 `.claude`、`.codex`、`.cursor` 或 `.opencode` 目录当作归 Slipway 所有。

## 保留用户自有文件

接受刷新 diff 之前，先检查相邻的 host 配置：

```bash
git status --short .claude .codex .cursor .opencode
```

生成的文件由 CLI 负责。用户自有的 host 设置、本地 prompt、手写命令以及非 Slipway 的 hook 都应保持原样。

如果刷新结果删除了某个旧的、由 Slipway 拥有的启动器或 prompt，提交前请确认新生成的接口已经存在。Codex 命令接口现在位于：

```text
.codex/skills/slipway-<command>/SKILL.md
```

## 改动命令或技能接口之后

如果你改动了命令注册、生成的技能、JSON 契约或文档 token，需要更新接口清单：

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go test ./internal/toolgen -run SurfaceManifest -count=1
```

清单是从 Go 权威源和文档 token 推导出来的。除非你是在修复生成器本身，否则不要手动编辑生成的行。

## 相关内容

- [AI 工具适配器](../reference/ai-tools.md)
- [命令](../reference/commands.md)
- [恢复与排障](recover-and-troubleshoot.md)
