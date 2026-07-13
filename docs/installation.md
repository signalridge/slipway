# Installation

Primary installation paths are Go, GitHub Release archives and Linux packages, the GHCR container image, and the repository Nix flake. Homebrew, Scoop, and AUR are optional channels described below.

## Go

```bash
go install github.com/signalridge/slipway@latest
```

## Direct archive

Download the archive for your OS and architecture from [GitHub Releases](https://github.com/signalridge/slipway/releases), download `checksums.txt`, verify the archive, extract it, and place `slipway` on `PATH`. Linux and macOS archives use `.tar.gz`; Windows archives use `.zip`.

For example, the following installs the latest Linux `amd64` archive:

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

Use `darwin` and the appropriate architecture for macOS (with `shasum -a 256 --check -` in place of `sha256sum`), or verify the Windows `.zip` hash from `checksums.txt` with `Get-FileHash` before using `Expand-Archive` and moving `slipway.exe` to a directory on `PATH`.

## Linux packages

Download the package for your architecture from [GitHub Releases](https://github.com/signalridge/slipway/releases), then run the command for your distribution from the download directory:

```bash
# Debian or Ubuntu
sudo apt install ./slipway*.deb

# Fedora, RHEL, or another RPM-based distribution
sudo dnf install ./slipway*.rpm

# Alpine
sudo apk add --allow-untrusted ./slipway*.apk
```

## Container

Versioned container images are published to [GitHub Container Registry](https://github.com/signalridge/slipway/pkgs/container/slipway):

```bash
docker pull ghcr.io/signalridge/slipway:<version>
docker run --rm ghcr.io/signalridge/slipway:<version> --version
```

The image includes Git because repository discovery and run observation depend on it. On Linux, run with your host UID/GID when the command must write capabilities or journals into a mounted worktree:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/signalridge/slipway:<version> install --tool claude
```

## Nix

The repository flake supports one-off runs and profile installation:

```bash
nix run github:signalridge/slipway
nix profile install github:signalridge/slipway
```

## Optional package-manager channels

Homebrew, Scoop, and AUR are optional release outputs. GoReleaser uses `skip_upload: auto` for them: Homebrew and Scoop publishing requires the `GH_PAT` secret, while AUR publishing requires an AUR SSH key. A release may therefore omit updates to these channels when the corresponding secret is unavailable; this does not make them release gates.

### Homebrew cask

```bash
brew install --cask signalridge/tap/slipway
```

### Scoop

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install signalridge/slipway
```

### AUR

Install the optional `slipway-bin` package with an AUR helper:

```bash
yay -S slipway-bin
```

## Install host capabilities

Run from any directory inside a Git worktree:

```bash
slipway install --tool claude
```

Use repeated `--tool`, a comma-separated value, or `--tool all`. Without `--tool`, Slipway installs adapters whose host directories it detects. `--refresh` updates only files whose ownership hash still matches.

Supported IDs are `claude`, `codex`, `copilot`, `cursor`, `kilo`, `kiro`, `opencode`, `pi`, `qwen`, and `windsurf`.

```bash
slipway list
slipway doctor
```

## Safe refresh and removal

Generated files are recorded in a per-host ownership manifest. Refresh and uninstall preserve user-modified or unknown files and report them. Paths that escape the host area, duplicate claims, malformed hashes, and symlink traversal fail safely.

```bash
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

The only accepted manifest format is version 2. Any other version fails closed and cannot authorize install, refresh, uninstall, or list. A marker without a current manifest establishes no ownership and leaves the adapter surface unchanged. Host settings are outside adapter ownership and are never modified.

No SessionStart or prompt-submission activation is installed.
