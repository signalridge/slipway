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

The only accepted manifest format is version 2. Any other version fails closed and cannot authorize install, refresh, uninstall, or list. A marker without a current manifest establishes no ownership and leaves the adapter surface unchanged. Host settings are outside adapter ownership and are never modified.

No SessionStart or prompt-submission activation is installed.
