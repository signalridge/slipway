# 安装

仓库当前接口与各安装渠道中的最新软件包不一定同步。阅读其余文档前，请确认 `slipway --help` 包含 `install`、`uninstall`、`list`、`doctor`、`run`、`status` 和 `stop` 七个命令。

## 构建当前 checkout

使用 [`go.mod`](https://github.com/signalridge/slipway/blob/main/go.mod) 声明的 Go 版本（当前为 Go 1.26.5 或更高）：

```bash
go build -o ./slipway .
./slipway --help
```

这是评估尚未发布的仓库 revision 最可靠的方式。

## Tag 版本

请选择 release notes 已包含七命令软自动驾驶接口的 tag。核心产物发布在 [GitHub Releases](https://github.com/signalridge/slipway/releases)：

- Linux 和 macOS 的 `.tar.gz`；
- Windows 的 `.zip`；
- `.deb`、`.rpm` 与 `.apk` Linux 软件包；
- `checksums.txt` 与 SBOM；
- `ghcr.io/signalridge/slipway` 下的版本化镜像。

SLSA provenance 由独立的 post-release job 生成，并在该 job 成功时追加到 release。缺少 provenance 不表示已经发布的 core archive、package、checksum 或 SBOM 从未创建；请验证所选 tag 下实际存在的 artifact。

下载 archive 和 `checksums.txt`，校验后再解压，并将 `slipway`（Windows 为 `slipway.exe`）放入 `PATH`。

Linux 软件包可在下载目录中安装：

```bash
# Debian 或 Ubuntu
sudo apt install ./slipway*.deb

# Fedora、RHEL 或其他 RPM 发行版
sudo dnf install ./slipway*.rpm

# Alpine
sudo apk add --allow-untrusted ./slipway*.apk
```

安装后检查接口：

```bash
slipway --version
slipway --help
```

## 从 tag 使用 Go 安装

在 latest release 包含这套接口前，不要使用 `@latest`；请固定兼容 tag：

```bash
go install github.com/signalridge/slipway@vX.Y.Z
```

`go install` 构建的二进制可能因缺少 release linker flags 而显示开发版本信息；兼容性应以固定的 module 版本和命令树为准。

## 容器

```bash
docker pull ghcr.io/signalridge/slipway:vX.Y.Z
docker run --rm ghcr.io/signalridge/slipway:vX.Y.Z --help
```

镜像内包含 Git。在 Linux 挂载 worktree 并写入能力或 Run 数据时，使用宿主 UID/GID：

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/signalridge/slipway:vX.Y.Z install --tool claude
```

## Nix

将 flake 固定到兼容 tag。未指定 tag 的 GitHub flake 会跟随可变的默认分支。

```bash
nix run github:signalridge/slipway/vX.Y.Z -- --help
nix profile install github:signalridge/slipway/vX.Y.Z
```

## 可选包管理渠道

Homebrew、Scoop 与 AUR 是次级发布渠道，可能落后于 GitHub 核心 release。安装后请检查显示版本，并执行 `slipway --help`。

### Homebrew cask

Release workflow 验证的是显式 tap 与 trust 流程：

```bash
brew tap signalridge/tap
brew trust signalridge/tap
brew install --cask slipway
```

### Scoop

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install signalridge/slipway
```

### AUR

```bash
yay -S slipway-bin
```

## 安装宿主能力

在目标 Git worktree 内执行：

下列命令使用当前 checkout 构建的 `./slipway`。如果安装的是兼容的 tagged package，请改用 `PATH` 中的 `slipway`。

```bash
./slipway install --tool claude
./slipway list
./slipway doctor
```

支持的 ID 为 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf`。可重复 `--tool` 选择多个宿主，也可以传入一个逗号分隔的值，例如 `--tool claude,codex`。

Kiro 首次安装需要显式指定 surface：

```bash
./slipway install --tool kiro --surface ide   # 或：--surface cli
```

Kiro 位于混合选择中时，`--surface` 只作用于 Kiro；例如 `--tool claude --tool kiro --surface ide` 和 `--tool all --surface ide` 都合法。Refresh 和 uninstall 会自动沿用已记录的 Kiro surface。

省略 `--tool` 时，Slipway 根据宿主目录进行检测。检测只是一项便利功能；在配置多个宿主的仓库中，安装前先检查 `./slipway list`。

## 刷新与卸载

```bash
./slipway install --tool claude --refresh
./slipway uninstall --tool claude
```

Slipway 在每个宿主的 ownership manifest 中记录生成路径和 hash。Refresh 与 uninstall 只修改仍与记录相符的 managed file。被用户修改、未知、格式错误、越界或经过 symlink 的路径会被保留或拒绝并报告；宿主设置不属于 adapter ownership。

若当前 manifest 仍 claim 由较早 release 生成的 bytes，refresh 和 uninstall 会保留该文件并撤销 stale claim，而不会把它视为可以安全覆盖或删除。检查并移走保留的文件；若希望当前 release 重新生成它，再运行 `slipway install --refresh`。

不要通过伪造或修改 ownership manifest 来恢复安装。若当前 manifest 已丢失，但 `.adapter-generated` 或外观上像生成物的文件仍在，请先备份并检查宿主表面。只移走 sentinel 和你明确希望 Slipway 重建的文件，再针对该宿主重新运行 `slipway install`。留在原处的内容会继续被保留且绝不被收编。这是手工恢复，不是 manifest 重建或自动迁移。

移除 adapter 不会删除 Run journal。Run 保留规则见 [Run、恢复与隐私](guides/runs-and-recovery.md)。
