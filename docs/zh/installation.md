# 安装

通常应当从已发布的发布产物或基于发布产物的包管理渠道来安装 Slipway。面向开发者的安装方式，例如 `go install`、Nix 和本地源码构建，则适用于以下场景：你需要尚未发布的版本、某个还没有打包的平台路径，或一个可复现的开发环境。

把 `vX.Y.Z` 替换成你想要的发布标签。对于尚未发布的工作内容，请使用本地 checkout 构建方式。

## 官方来源

只使用 Slipway 项目自己维护的官方发布来源：`signalridge/slipway` 下的 GitHub Releases、`ghcr.io/signalridge/slipway` 的容器镜像、来自 `signalridge/tap` 的 Homebrew Cask 条目、来自 `signalridge/scoop-bucket` 的 Scoop manifest，以及该渠道已发布时的 AUR `slipway-bin`。如果某个 AI 工具在其他 registry 中发现了同名包，请先停下来核实其归属，再决定是否安装。

## 前置条件

- Git：用于仓库初始化和受治理的工作。
- Go：从源码构建或使用 `go install` 时，版本需与 `go.mod` 匹配。
- 可选：使用 flake 包时需要 Nix。
- 可选：使用容器镜像时需要 Docker 或其他 OCI 运行时。
- 可选：本地构建文档时需要 Astro Starlight。
- 可选：一个或多个受 `slipway init --tools` 支持的 AI 编码工具。

## 安装顺序

常规安装请按以下顺序：

1. 优先选择适配你平台的官方发布归档包，或基于发布产物的包管理渠道。
2. 当你想运行 Slipway 而不在主机上安装二进制时，使用容器镜像。
3. 当没有可用的发布包，或你明确想要一个由 Go 管理的二进制时，使用 `go install`。
4. 进行开发和处理尚未发布的改动时，使用 Nix 或本地源码构建。

## 发布安装矩阵

| 平台 | 发布产物 | 包管理渠道 | 其他方式 |
| --- | --- | --- | --- |
| macOS amd64 | `slipway_<version>_darwin_amd64.tar.gz` | 已发布时使用 Homebrew Cask | Go install、Nix、源码构建 |
| macOS arm64 | `slipway_<version>_darwin_arm64.tar.gz` | 已发布时使用 Homebrew Cask | Go install、Nix、源码构建 |
| Linux amd64 | `slipway_<version>_linux_amd64.tar.gz`、`.deb`、`.rpm`、`.apk` | 已发布时使用 AUR `slipway-bin` | Go install、Nix、容器镜像、源码构建 |
| Linux arm64 | `slipway_<version>_linux_arm64.tar.gz`、`.deb`、`.rpm`、`.apk` | 已发布时使用 AUR `slipway-bin` | Go install、Nix、容器镜像、源码构建 |
| Windows amd64 | `slipway_<version>_windows_amd64.zip` | 已发布时使用 Scoop | Go install、源码构建 |
| Windows arm64 | `slipway_<version>_windows_arm64.zip` | 已发布时使用 Scoop | Go install、源码构建 |

发布工作流完成后，GoReleaser 还会发布 `checksums.txt`、归档包的 SBOM、校验和签名以及容器签名。包管理渠道使用可选的发布凭据；如果某次发布没有对应渠道，请先选用直接的发布归档包，再考虑回退到 `go install`、Nix 或本地 checkout 方式。

## 直接使用发布归档包

当你想拿到已发布的二进制而不经过包管理器时，使用直接的发布归档包。下面各平台小节给出了 macOS、Linux 和 Windows 的命令。

## 包管理器

当某次发布已同步到对应渠道时，使用基于发布产物的包管理器：

- macOS：通过 `signalridge/tap` 使用 Homebrew Cask。
- Linux：`.deb`、`.rpm`、`.apk` 或 AUR `slipway-bin`。
- Windows：通过 `signalridge/scoop-bucket` 使用 Scoop。

## Go Install 回退方案

当你已安装 Go，并想要一个开发者回退方案或一个在 `PATH` 上、由 Go 管理的二进制时，使用这种方式：

```bash
go install github.com/signalridge/slipway@latest
slipway --version
```

安装特定发布版本：

```bash
go install github.com/signalridge/slipway@vX.Y.Z
slipway --version
```

## 从源码构建

仅在开发 Slipway 或测试尚未发布的改动时使用这种方式：

```bash
go build -o ./bin/slipway .
./bin/slipway --version
./bin/slipway --help
```

直接使用 `./bin/slipway`，或把 `./bin` 加入你的 `PATH`。

## macOS

Homebrew Cask：

```bash
brew install --cask signalridge/tap/slipway
slipway --version
```

直接使用归档包：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64) SLIPWAY_ARCH=arm64 ;;
  x86_64) SLIPWAY_ARCH=amd64 ;;
  *) echo "unsupported macOS arch: $ARCH" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_darwin_${SLIPWAY_ARCH}.tar.gz"
tar xzf "slipway_${VERSION}_darwin_${SLIPWAY_ARCH}.tar.gz"
install -m 0755 slipway /usr/local/bin/slipway
slipway --version
```

## Linux

直接使用归档包：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
ARCH="$(uname -m)"
case "$ARCH" in
  aarch64|arm64) SLIPWAY_ARCH=arm64 ;;
  x86_64) SLIPWAY_ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $ARCH" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${SLIPWAY_ARCH}.tar.gz"
tar xzf "slipway_${VERSION}_linux_${SLIPWAY_ARCH}.tar.gz"
sudo install -m 0755 slipway /usr/local/bin/slipway
slipway --version
```

Debian 或 Ubuntu：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.deb"
sudo dpkg -i "slipway_${VERSION}_linux_${ARCH}.deb"
slipway --version
```

Fedora、RHEL 或兼容的 RPM 系统：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.rpm"
sudo rpm -i "slipway_${VERSION}_linux_${ARCH}.rpm"
slipway --version
```

Alpine：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.apk"
sudo apk add --allow-untrusted "slipway_${VERSION}_linux_${ARCH}.apk"
slipway --version
```

包已发布时，通过 AUR 在 Arch Linux 上安装：

```bash
yay -S slipway-bin
slipway --version
```

容器镜像：

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
docker run --rm ghcr.io/signalridge/slipway:${VERSION} --version
```

在容器中对当前仓库进行操作：

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:${VERSION} status --json
```

## Windows

bucket 已发布时，使用 Scoop：

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install slipway
slipway --version
```

直接使用 zip：

```powershell
$Tag = "vX.Y.Z"
$Version = $Tag.TrimStart("v")
$Arch = "amd64"
$Asset = "slipway_${Version}_windows_${Arch}.zip"
Invoke-WebRequest "https://github.com/signalridge/slipway/releases/download/${Tag}/${Asset}" -OutFile $Asset
Expand-Archive $Asset -DestinationPath .
.\slipway.exe --version
```

在 Windows arm64 上，当存在对应的发布产物时，用 `arm64` 替换 `amd64`。

## Nix

从 checkout 构建：

```bash
nix build .#slipway
./result/bin/slipway --version
```

从 GitHub 运行：

```bash
nix run github:signalridge/slipway#slipway -- --help
```

## 校验发布下载

当你的环境要求做发布产物完整性校验时，连同资产一起下载发布校验和文件，并在安装前完成校验：

```bash
TAG=vX.Y.Z
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
```

在 macOS 上，如果没有 GNU 的 `sha256sum`，请使用 `shasum -a 256`。

## 初始化仓库

在目标仓库或其子目录中运行 `init`：

```bash
slipway init
```

这会写入仓库的 `.slipway.yaml` 配置，并在 `.gitignore` 中加入一个受管理的
"# Slipway local state (managed)" 块（忽略 bundle 本地的 `events/`、`verification/`、旧版的逐变更 `evidence/` 以及 `.worktrees/` 路径），同时创建仓库本地的 `.git/slipway/` 运行时区域。运行时的任务证据记录在 `.git/slipway/runtime/changes/<slug>/evidence/` 下。除非你传入 `--tools`，否则它不会生成任何 AI 工具的接入面：

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools copilot,pi,qwen,windsurf
slipway init --tools all
slipway init --tools none
```

支持的工具 ID 有 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen` 和 `windsurf`。

具有代表性的生成适配器目录包括 `.claude/skills`、`.codex/skills`、`.github/skills`、`.cursor/skills`、`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、`.qwen/skills` 和 `.windsurf/skills`。Copilot 还会在 `.github/prompts` 下写入命令提示词，并把生成的归属状态保存在 `.github/copilot/slipway`。

使用 `--refresh` 重新生成由 Slipway 管理的适配器文件：

```bash
slipway init --tools opencode --refresh
```

如果刷新时省略 `--tools`，Slipway 会检测之前生成的适配器并刷新这些受管理的接入面。刷新还会清理 Slipway 自有的旧版 shell hook 启动器和 settings 条目，同时保留用户自有的 hook。

## AI 工具安装提示词

当你想让某个 AI 编码工具为当前仓库安装并初始化 Slipway 时，把下面这段内容粘贴进去。粘贴前请先阅读它，并在 agent 运行时全程监督。这段提示词有意保持简短——它把 agent 指向本页面，让权威指引集中在一处：

```text
为这个仓库安装 Slipway。

阅读 https://signalridge.github.io/slipway/installation/ ，特别是
"AI 工具安装提示词" 小节，并按其中说明操作。

安装前先检测操作系统和 CPU 架构，并运行 `slipway --version` 检查
Slipway 是否已经在 PATH 上。优先使用文档列出的、由 Slipway 项目
（`signalridge` 组织）维护的发布来源。不要从无关 registry 安装同名包。
如果没有适用的文档路径，停止并报告。

安装后运行 `slipway --version`、`slipway status --json` 和
`git status --short --branch`。报告哪条安装路径成功，以及生成了哪些文件
（尤其是 `.slipway.yaml` 和所选工具 ID 对应的适配器目录）。
```

本节其余内容就是 agent 在抓取本页面后会读到的权威指引。

### 探查

- 检查仓库根目录，记录 `.slipway.yaml` 是否已存在。
- 检测本机的操作系统和 CPU 架构。
- 运行 `slipway --version`。如果它打印出版本号，说明 Slipway 已在 PATH 上——跳到 **校验**。否则继续 **安装**。

### 安装（按优先级顺序尝试；首次成功即停止）

1. 适配本机操作系统和架构的、由 Slipway 项目（`signalridge`）维护的官方发布产物或基于发布产物的包管理渠道。如果缺少对应产物，不要回退到其他无关 registry 的同名包——继续下一步。
2. **macOS：** 如果有 `brew` 且 `signalridge/tap` cask 已发布，运行 `brew install --cask signalridge/tap/slipway`。否则使用对应的 `darwin_amd64` 或 `darwin_arm64` 发布归档包。
3. **Linux：** 选用对应的 `linux_amd64` 或 `linux_arm64` 发布归档包，或在对应渠道可用时选用 `.deb`、`.rpm`、`.apk`、AUR `slipway-bin` 或 `ghcr.io/signalridge/slipway` 容器镜像。
4. **Windows：** 如果已配置 Scoop（`signalridge/scoop-bucket`）就用它。否则使用对应的 `windows_amd64` 或 `windows_arm64` 发布 zip。
5. 如果没有可用的基于发布产物的渠道，但已安装 Go，运行 `go install github.com/signalridge/slipway@latest`。
6. 如果本仓库就是 Slipway 源码 checkout，并且你确实需要本地尚未发布的版本，运行 `go build -o ./bin/slipway .` 并使用 `./bin/slipway`。
7. 如果以上官方方式都行不通，停止并报告尝试了哪些方式、各自卡在哪里。不要自创安装器，也不要从无关 registry 拉取同名包。

### 初始化

- 如果不清楚本仓库使用哪些 AI 工具适配器，先询问。支持的工具 ID 有 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen` 和 `windsurf`。
- 运行 `slipway init --tools <tool-id>`、`slipway init --tools claude,codex,opencode`、`slipway init --tools copilot,kiro,pi,qwen,windsurf,kilo` 或 `slipway init --tools all` 之一。
- 如果 Slipway 生成的适配器文件已存在，改用 `slipway init --tools <detected-tools> --refresh`。
- 不要覆盖无关的、用户自有的 AI 工具文件。如果某个生成路径会与用户自有内容冲突，停止并报告，不要覆盖。

### 校验

- `slipway --version`
- `slipway status --json`
- `git status --short --branch`

### 报告

- 哪条安装路径成功了，以及哪些更靠前的路径被跳过或失败。
- 新生成的文件，尤其是 `.slipway.yaml` 以及所选的适配器目录，例如 `.claude/skills`、`.codex/skills`、`.github/skills`、`.cursor/skills`、`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、`.qwen/skills` 或 `.windsurf/skills`。
- 用户需要知道的任何未决后续事项（例如本平台缺少对应发布版本，或 `slipway init` 的某些选择仍需人来拍板）。

具体到 OpenCode，预期生成的项目接入面是：

- `.opencode/skills/slipway-*/SKILL.md`
- `.opencode/commands/slipway-*.md`
- `.opencode/hooks/slipway-session-start`
- `.opencode/hooks/slipway-session-start.ps1`
- `.opencode/hooks/slipway-session-start.cmd`

OpenCode 命令采用斜杠加连字符的写法，例如 `/slipway-new`、`/slipway-next` 和 `/slipway-run`。某些 OpenCode 版本会在命令选择器里给项目命令加上项目前缀；生成的文件路径才是稳定的契约。

使用生成 hook 启动器的适配器（包括 Cursor 和 OpenCode）会在各自的 `hooks/` 目录下收到针对 POSIX、PowerShell 和 `cmd.exe` 的原生启动器文件。支持 settings 的 hook 宿主（Claude 和 Qwen）则改为在 `settings.json` 中直接注册裸的内联 `slipway hook ...` 命令，不生成启动器文件。Pi 的 settings 注册的是 skill 和提示词，而非 hook。无论哪种方式，生成的 hook 都不需要 bash、Python、`jq`、`gh` 或 Go 运行时。

生成的 skill 辅助命令通过 `slipway tool ...` 运行，而不是生成的脚本载荷。手动辅助命令可能仍需要明确的已认证后端或领域工具，例如用于 GitHub 辅助的 `gh`，或用于 Go 测试污染追踪的 `go`，当这些不可用时会失败即停并给出补救建议。

## 校验安装

```bash
slipway --version
slipway status --json
git status --short --branch
```

在已用适配器初始化的仓库中，检查生成的文件：

```bash
find .claude .codex .github/skills .github/prompts .github/copilot .cursor .kilocode .kiro .opencode .pi .qwen .windsurf -maxdepth 3 -type f 2>/dev/null
```

Codex 的命令接入面会作为 skill 生成在 `.codex/skills/slipway-<command>/SKILL.md` 下。Codex 刷新只管理项目本地的 `.codex/` 适配器树；它不会触碰宿主全局的 `$CODEX_HOME/prompts/` 或 `~/.codex/prompts/` 文件。对于支持 hook 的适配器，`--refresh` 会移除 Slipway 自有的、已退役的 hook 启动器。支持 settings 的宿主会把已退役的启动器路径 settings 条目迁移为裸的内联 `slipway hook ...` 命令；Cursor 和 OpenCode 则保留按文件路径的 session-start 启动器。
