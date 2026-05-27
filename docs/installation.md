# Installation

Slipway should normally be installed from published release artifacts or release-backed package channels. Developer-oriented paths such as `go install`, Nix, and local source builds are available when you need an unreleased version, a not-yet-packaged platform path, or a reproducible development environment.

Replace `vX.Y.Z` with the release tag you want. For unreleased work, use the local checkout build path.

## Official Sources

Use documented release sources owned by the Slipway project: GitHub Releases under `signalridge/slipway`, container images at `ghcr.io/signalridge/slipway`, Homebrew Cask entries from `signalridge/tap`, Scoop manifests from `signalridge/scoop-bucket`, and AUR `slipway-bin` when that channel has been published. If an AI tool finds a same-name package in another registry, stop and verify ownership before installing it.

## Prerequisites

- Git for repository initialization and governed work.
- Go matching `go.mod` when building from source or using `go install`.
- Optional: Nix when using the flake package.
- Optional: Docker or another OCI runtime when using the container image.
- Optional: MkDocs Material for local docs builds.
- Optional: one or more AI coding tools supported by `slipway init --tools`.

## Install Order

Use this order for normal installations:

1. Prefer an official release archive or a release-backed package channel for your platform.
2. Use the container image when you want to run Slipway without installing a host binary.
3. Use `go install` when release packages are unavailable or you explicitly want a Go-managed binary.
4. Use Nix or a local source build for development and unreleased changes.

## Release Install Matrix

| Platform | Release artifacts | Package channels | Other paths |
| --- | --- | --- | --- |
| macOS amd64 | `slipway_<version>_darwin_amd64.tar.gz` | Homebrew Cask when published | Go install, Nix, source build |
| macOS arm64 | `slipway_<version>_darwin_arm64.tar.gz` | Homebrew Cask when published | Go install, Nix, source build |
| Linux amd64 | `slipway_<version>_linux_amd64.tar.gz`, `.deb`, `.rpm`, `.apk` | AUR `slipway-bin` when published | Go install, Nix, container image, source build |
| Linux arm64 | `slipway_<version>_linux_arm64.tar.gz`, `.deb`, `.rpm`, `.apk` | AUR `slipway-bin` when published | Go install, Nix, container image, source build |
| Windows amd64 | `slipway_<version>_windows_amd64.zip` | Scoop when published | Go install, source build |
| Windows arm64 | `slipway_<version>_windows_arm64.zip` | Scoop when published | Go install, source build |

GoReleaser also publishes `checksums.txt`, archive SBOMs, checksum signatures, and container signatures when the release workflow completes. Package-manager channels use optional publishing credentials; if a channel is not present for a release, prefer the direct release archive before falling back to `go install`, Nix, or the local checkout path.

## Direct Release Archives

Use a direct release archive when you want the published binary without a package manager. The per-platform sections below show macOS, Linux, and Windows commands.

## Package Managers

Use release-backed package managers when the matching channel has been published for the release:

- macOS: Homebrew Cask through `signalridge/tap`.
- Linux: `.deb`, `.rpm`, `.apk`, or AUR `slipway-bin`.
- Windows: Scoop through `signalridge/scoop-bucket`.

## Go Install Fallback

Use this path when Go is available and you want a developer fallback or a Go-managed binary on `PATH`:

```bash
go install github.com/signalridge/slipway@latest
slipway --version
```

For a specific release:

```bash
go install github.com/signalridge/slipway@vX.Y.Z
slipway --version
```

## Build From Source

Use this path only when developing Slipway or testing unreleased changes:

```bash
go build -o ./bin/slipway .
./bin/slipway --version
./bin/slipway --help
```

Use `./bin/slipway` directly or put `./bin` on your `PATH`.

## macOS

Homebrew Cask:

```bash
brew install --cask signalridge/tap/slipway
slipway --version
```

Direct archive:

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

Direct archive:

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

Debian or Ubuntu:

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

Fedora, RHEL, or compatible RPM systems:

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

Alpine:

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

Arch Linux through AUR when the package has been published:

```bash
yay -S slipway-bin
slipway --version
```

Container image:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
docker run --rm ghcr.io/signalridge/slipway:${VERSION} --version
```

To operate on the current repository from the container:

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:${VERSION} status --json
```

## Windows

Scoop, when the bucket has been published:

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install slipway
slipway --version
```

Direct zip:

```powershell
$Tag = "vX.Y.Z"
$Version = $Tag.TrimStart("v")
$Arch = "amd64"
$Asset = "slipway_${Version}_windows_${Arch}.zip"
Invoke-WebRequest "https://github.com/signalridge/slipway/releases/download/${Tag}/${Asset}" -OutFile $Asset
Expand-Archive $Asset -DestinationPath .
.\slipway.exe --version
```

Use `arm64` instead of `amd64` on Windows arm64 when that release asset is present.

## Nix

From a checkout:

```bash
nix build .#slipway
./result/bin/slipway --version
```

From GitHub:

```bash
nix run github:signalridge/slipway#slipway -- --help
```

## Verify Release Downloads

Download the release checksum file with the asset and verify before installing when your environment requires artifact integrity checks:

```bash
TAG=vX.Y.Z
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
```

On macOS, use `shasum -a 256` if GNU `sha256sum` is unavailable.

## Initialize A Repository

Run `init` from the target repository or a child directory inside it:

```bash
slipway init
```

This writes `.slipway.yaml` only. Add AI-tool surfaces with `--tools`:

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools all
slipway init --tools none
```

Supported tool IDs are `claude`, `codex`, `cursor`, `gemini`, and `opencode`.

Use `--refresh` to regenerate Slipway-managed adapter files:

```bash
slipway init --tools opencode --refresh
```

If `--tools` is omitted during refresh, Slipway detects previously generated adapters and refreshes those managed surfaces.

## AI Tool Installation Prompt

Paste this into an AI coding tool when you want the tool to install and initialize Slipway for the current repository:

```text
Install Slipway for this repository.

Work from the current Git checkout. First inspect the repository root, confirm whether .slipway.yaml already exists, identify the operating system and CPU architecture, and check whether a slipway binary is already on PATH with `slipway --version`.

If Slipway is not installed, choose a documented path that fits this machine:
1. Prefer a published Slipway release artifact or release-backed package channel owned by the Slipway project for this OS and architecture. Do not install same-name packages from unrelated registries without verifying ownership.
2. On macOS, use Homebrew Cask if `brew` is available and `signalridge/tap/slipway` has been published: `brew install --cask signalridge/tap/slipway`; otherwise use the matching `darwin_amd64` or `darwin_arm64` release archive.
3. On Linux, use the matching `linux_amd64` or `linux_arm64` release archive, or the matching `.deb`, `.rpm`, `.apk`, AUR package, or container image when that channel is available.
4. On Windows, use Scoop if available and configured, otherwise use the matching `windows_amd64` or `windows_arm64` release zip.
5. If release packages are unavailable but Go is available, run `go install github.com/signalridge/slipway@latest` and then `slipway --version`.
6. If this repository is the Slipway source checkout and you intentionally need the local unreleased version, run `go build -o ./bin/slipway .` and use `./bin/slipway`.
7. If none of the documented paths work, stop and report the missing prerequisite instead of inventing an installer.

Initialize the current repository with the tool adapters I use. Ask if the tool list is unclear. Supported tool IDs are claude, codex, cursor, gemini, and opencode.

Use one of:
- `slipway init --tools <tool-id>`
- `slipway init --tools claude,codex,opencode`
- `slipway init --tools all`

Do not overwrite unrelated user-owned AI-tool files. If Slipway-generated adapter files already exist, use `slipway init --tools <detected-tools> --refresh`.

Verify the result by running:
- `slipway status --json`
- `git status --short --branch`

Report the generated files, especially .slipway.yaml and any tool directories such as .opencode/skills or .opencode/commands.
```

For OpenCode specifically, the expected generated project surfaces are:

- `.opencode/skills/slipway-*/SKILL.md`
- `.opencode/commands/slipway-*.md`
- `.opencode/hooks/slipway-session-start.sh`

OpenCode commands use slash-hyphen spelling such as `/slipway-new`, `/slipway-next`, and `/slipway-run`. Some OpenCode builds display project commands with a project prefix in the command picker; the generated file path is the stable contract.

## Verify Installation

```bash
slipway --version
slipway status --json
git status --short --branch
```

In a repository initialized with adapters, inspect generated files:

```bash
find .claude .codex .cursor .gemini .opencode -maxdepth 3 -type f 2>/dev/null
```

Codex prompts are generated in `$CODEX_HOME/prompts/` if `CODEX_HOME` is set, otherwise under `~/.codex/prompts/`.
