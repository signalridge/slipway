# Installation

## Go

```bash
go install github.com/signalridge/slipway@latest
```

Release archives and packages are published on [GitHub Releases](https://github.com/signalridge/slipway/releases). Container and Nix builds remain available through the repository release assets and flake.

## Container

The published image includes Git because repository discovery and run observation depend on it. On Linux, run with your host UID/GID when the command must write capabilities or journals into a mounted worktree:

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/signalridge/slipway:<version> install --tool claude
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

The current manifest format is version 2. Version 1 is read only to remove pristine files produced by the retired adapter. Marker-only legacy state is not sufficient proof for deletion. Retired managed Claude/Qwen hook entries and the bounded Codex configuration block are removed precisely; unrelated settings remain.

No SessionStart or prompt-submission activation is installed.
