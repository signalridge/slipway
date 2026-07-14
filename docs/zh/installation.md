# 安装

主要安装方式是 Go、[GitHub Releases](https://github.com/signalridge/slipway/releases) 的直接压缩包和 Linux 软件包、GHCR 容器镜像，以及仓库的 Nix flake。Homebrew、Scoop 和 AUR 是可选渠道。

## Go

```bash
go install github.com/signalridge/slipway@latest
```

## 直接压缩包

从 GitHub Releases 下载对应 OS/架构的压缩包和 `checksums.txt`，校验后解压，并把二进制文件放入 `PATH`。以下是 Linux `amd64` 示例：

```bash
release_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/signalridge/slipway/releases/latest)"
tag="${release_url##*/}"
version="${tag#v}"
archive="slipway_${version}_linux_amd64.tar.gz"
base_url="https://github.com/signalridge/slipway/releases/download/${tag}"
curl -fLO "${base_url}/${archive}"
curl -fLO "${base_url}/checksums.txt"
grep -F " ${archive}" checksums.txt | sha256sum --check -
tar -xzf "${archive}"
sudo install -m 0755 slipway /usr/local/bin/slipway
```

macOS 请选择 `darwin` 的 `.tar.gz`；Windows 请选择 `.zip`，先用 `Get-FileHash` 与 `checksums.txt` 对照，再用 `Expand-Archive` 解压。

## Linux 软件包

下载对应架构的软件包后，在下载目录运行适合当前发行版的命令：

```bash
sudo apt install ./slipway*.deb
sudo dnf install ./slipway*.rpm
sudo apk add --allow-untrusted ./slipway*.apk
```

## 容器

带版本号的镜像发布到 [GHCR](https://github.com/signalridge/slipway/pkgs/container/slipway)：

```bash
docker pull ghcr.io/signalridge/slipway:<version>
docker run --rm ghcr.io/signalridge/slipway:<version> --version
```

容器镜像包含 Git。Linux 上向挂载 worktree 写入 capability 或 journal 时，请传入宿主 UID/GID：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:<version> install --tool claude
```

## Nix

```bash
nix run github:signalridge/slipway
nix profile install github:signalridge/slipway
```

## 可选包管理渠道

Homebrew、Scoop 与 AUR 都是独立 best-effort 渠道。核心 release 显式跳过三者，先独立发布压缩包、Linux 软件包、checksum、SBOM、provenance 与容器；只有存在 `GH_PAT`（Homebrew/Scoop）或 AUR SSH key 时才运行单独可失败的发布/验证 job。每个 job 只有在可复现重建的 archive checksum 与已发布核心 release 完全一致后才更新渠道。Publisher、checksum 验证或渠道验证失败都不会阻塞或否定核心 release，因此可选渠道可能缺失或滞后；使用前请核对显示版本。

```bash
brew install --cask signalridge/tap/slipway
yay -S slipway-bin
```

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install signalridge/slipway
```

## 安装宿主 capability

```bash
slipway install --tool claude
slipway install --tool kiro --surface ide  # 或 --surface cli
```

支持 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf`；可以重复 `--tool` 或使用 `--tool all`。不指定时只安装检测到的宿主。Kiro 首次安装必须选择 `--surface ide|cli`，manifest 会记录该选择，后续 refresh/uninstall 自动沿用；其他宿主不能使用 `--surface`，已安装的 Kiro 也不会被静默切换。

Skill 宿主通过原生 skill UI 调用 `slipway-<name>`（Codex 为 `$slipway-<name>`，Pi 为 `/skill:slipway-<name>`）；Copilot 从 agent picker 选择 custom agent；Kilo、OpenCode、Windsurf 使用 `/slipway-<name>`；Kiro IDE 手动 include `#slipway-<name>`，Kiro CLI 使用 `kiro-cli chat --agent slipway-<name>`。Copilot 自动探测识别 `.github/copilot`、`.github/prompts` 或 `.github/skills` 任一目录，custom agent 写入 `.github/copilot/agents`。

```bash
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

Refresh 和 uninstall 只修改 current version 2 ownership manifest 中哈希仍匹配的文件。其他任何 manifest 版本都会在 install、refresh 或 uninstall 修改文件前 fail closed。只读 `list` 仍可执行：它把该宿主报告为未安装并附 advisory，同时继续完整列出其他宿主，且不修改文件系统。用户修改、未知、路径越界或 symlink 表面会被保留或安全拒绝；marker-only 不建立 ownership，也不会触发迁移或推断。宿主 settings 永不修改。不会安装 SessionStart 或 prompt-submit 自动入口。
