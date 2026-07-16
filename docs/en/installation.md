# Installation

The current repository interface and the latest package in every channel may not be the same. Before following the rest of the documentation, verify that `slipway --help` lists the seven commands `install`, `uninstall`, `list`, `doctor`, `run`, `status`, and `stop`.

## Build the current checkout

Use the Go version declared in [`go.mod`](https://github.com/signalridge/slipway/blob/main/go.mod) (currently Go 1.26.5 or newer):

```bash
go build -o ./slipway .
./slipway --help
```

This is the reliable way to evaluate an unreleased repository revision.

## Tagged releases

Choose a tag whose release notes include the seven-command soft-autopilot interface. Core release artifacts are published on [GitHub Releases](https://github.com/signalridge/slipway/releases):

- `.tar.gz` archives for Linux and macOS;
- `.zip` archives for Windows;
- `.deb`, `.rpm`, and `.apk` Linux packages;
- `checksums.txt`, SBOMs, and provenance;
- versioned images at `ghcr.io/signalridge/slipway`.

Download the archive and `checksums.txt`, verify the archive before extracting it, then place `slipway` (or `slipway.exe`) on `PATH`.

Linux packages can be installed from the download directory:

```bash
# Debian or Ubuntu
sudo apt install ./slipway*.deb

# Fedora, RHEL, or another RPM-based distribution
sudo dnf install ./slipway*.rpm

# Alpine
sudo apk add --allow-untrusted ./slipway*.apk
```

Verify the installed interface:

```bash
slipway --version
slipway --help
```

## Go installation from a tag

Do not use `@latest` until the latest release contains this interface. Pin a compatible tag:

```bash
go install github.com/signalridge/slipway@vX.Y.Z
```

A binary built with `go install` may show development version metadata because release linker flags are not present; use the pinned module version and command tree to establish compatibility.

## Container

```bash
docker pull ghcr.io/signalridge/slipway:vX.Y.Z
docker run --rm ghcr.io/signalridge/slipway:vX.Y.Z --help
```

The image includes Git. To install capabilities or create Run data in a mounted Linux worktree, use the host UID/GID:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/signalridge/slipway:vX.Y.Z install --tool claude
```

## Nix

Pin the flake to a compatible tag. An unqualified GitHub flake follows the repository's mutable default branch.

```bash
nix run github:signalridge/slipway/vX.Y.Z -- --help
nix profile install github:signalridge/slipway/vX.Y.Z
```

## Optional package-manager channels

Homebrew, Scoop, and AUR are secondary publishers and may lag the core GitHub release. Check the displayed version and run `slipway --help` after installation.

### Homebrew cask

The release workflow tests an explicit tap and trust sequence:

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

## Install host capabilities

Run from inside the target Git worktree:

The commands below use `./slipway`, the binary built from this checkout. If you installed a compatible tagged package, use the `slipway` binary on `PATH` instead.

```bash
./slipway install --tool claude
./slipway list
./slipway doctor
```

Supported IDs are `claude`, `codex`, `copilot`, `cursor`, `kilo`, `kiro`, `opencode`, `pi`, `qwen`, and `windsurf`. Repeat `--tool` to select several non-Kiro hosts.

Kiro must be installed separately the first time so its surface is unambiguous:

```bash
./slipway install --tool kiro --surface ide   # or: --surface cli
```

Do not combine `--surface` with non-Kiro selections. Once the Kiro surface is recorded, later refresh and uninstall infer it. A first-time `--tool all` selection also needs Kiro to have a recorded surface, so explicit per-host setup is clearer.

Without `--tool`, Slipway uses detected host directories. Detection is only a convenience; inspect `./slipway list` before installing into a repository with several host configurations.

## Refresh and uninstall

```bash
./slipway install --tool claude --refresh
./slipway uninstall --tool claude
```

Slipway records generated paths and hashes in a per-host ownership manifest. Refresh and uninstall mutate only matching managed files. Modified, unknown, malformed, out-of-host, or symlinked paths are preserved or rejected and reported; host settings remain outside adapter ownership.

Removing an adapter does not remove Run journals. See [Runs, recovery, and privacy](guides/runs-and-recovery.md) for Run retention.
