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
- Optional: Astro Starlight for local docs builds.
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

This writes the repo's `.slipway.yaml` config plus a managed
"# Slipway local state (managed)" block in `.gitignore` (ignoring bundle-local
`events/`, `verification/`, legacy per-change `evidence/`, and `.worktrees/`
paths), and creates the repo-local `.git/slipway/` runtime area. Runtime task
evidence is recorded under `.git/slipway/runtime/changes/<slug>/evidence/`. It
does not generate any AI-tool surfaces unless you pass `--tools`:

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools copilot,pi,qwen,windsurf
slipway init --tools all
slipway init --tools none
```

Supported tool IDs are `claude`, `codex`, `copilot`, `cursor`,
`kilo`, `kiro`, `opencode`, `pi`, `qwen`, and `windsurf`.

Representative generated adapter directories include `.claude/skills`,
`.codex/skills`, `.github/skills`, `.cursor/skills`,
`.kilocode/skills`, `.kiro/skills`, `.opencode/skills`, `.pi/skills`,
`.qwen/skills`, and `.windsurf/skills`. Copilot also writes command prompts
under `.github/prompts` and keeps its generated ownership state under
`.github/copilot/slipway`.

Use `--refresh` to regenerate Slipway-managed adapter files:

```bash
slipway init --tools opencode --refresh
```

If `--tools` is omitted during refresh, Slipway detects previously generated
adapters and refreshes those managed surfaces. Refresh also prunes
Slipway-owned legacy shell hook launchers and settings entries while preserving
user-owned hooks.

## AI Tool Installation Prompt

Paste this into an AI coding tool when you want the tool to install and initialize Slipway for the current repository. Read it before pasting and supervise the agent while it runs. The prompt is short on purpose — it points the agent at this page so the canonical guidance below stays in one place:

```text
Install Slipway for this repository.

Read https://signalridge.github.io/slipway/installation/ — specifically the
"AI Tool Installation Prompt" section — and follow it.

Before installing, detect the operating system and CPU architecture, and run
`slipway --version` to see if Slipway is already on PATH. Prefer documented
release sources owned by the Slipway project (the `signalridge` org). Do NOT
install same-name packages from unrelated registries. If no documented path
applies, stop and report.

After installing, run `slipway --version`, `slipway status --json`, and
`git status --short --branch`. Report which install path succeeded and what
files were generated (especially `.slipway.yaml` and adapter directories for
the selected tool IDs).
```

The rest of this section is the canonical guidance the agent will read after fetching this page.

### Discovery

- Inspect the repository root and note whether `.slipway.yaml` already exists.
- Detect this machine's operating system and CPU architecture.
- Run `slipway --version`. If it prints a version, Slipway is already on PATH — skip to **Verify**. Otherwise continue to **Install**.

### Install (try in preference order; stop on the first success)

1. A documented Slipway release artifact or release-backed package channel owned by the Slipway project (`signalridge`) for this OS and architecture. If the matching artifact is missing, do NOT fall back to a same-name package from an unrelated registry — continue to the next step.
2. **macOS:** if `brew` is available and the `signalridge/tap` cask has been published, run `brew install --cask signalridge/tap/slipway`. Otherwise use the matching `darwin_amd64` or `darwin_arm64` release archive.
3. **Linux:** pick the matching `linux_amd64` or `linux_arm64` release archive, or the matching `.deb`, `.rpm`, `.apk`, AUR `slipway-bin`, or `ghcr.io/signalridge/slipway` container image when that channel is available.
4. **Windows:** use Scoop (`signalridge/scoop-bucket`) if configured. Otherwise use the matching `windows_amd64` or `windows_arm64` release zip.
5. If no release-backed channel is available but Go is installed, run `go install github.com/signalridge/slipway@latest`.
6. If this repository IS the Slipway source checkout and you intentionally need the local unreleased version, run `go build -o ./bin/slipway .` and use `./bin/slipway`.
7. If none of the documented paths work, STOP and report which paths were attempted and what blocked each. Do not invent an installer and do not pull a same-name package from an unrelated registry.

### Initialize

- Ask which AI-tool adapters this repository uses if it is unclear. Supported tool IDs are `claude`, `codex`, `copilot`, `cursor`, `kilo`, `kiro`, `opencode`, `pi`, `qwen`, and `windsurf`.
- Run one of `slipway init --tools <tool-id>`, `slipway init --tools claude,codex,opencode`, `slipway init --tools copilot,kiro,pi,qwen,windsurf,kilo`, or `slipway init --tools all`.
- If Slipway-generated adapter files already exist, use `slipway init --tools <detected-tools> --refresh` instead.
- Do NOT overwrite unrelated user-owned AI-tool files. If a generated path would collide with user-owned content, stop and report instead of overwriting.

### Verify

- `slipway --version`
- `slipway status --json`
- `git status --short --branch`

### Report

- Which install path succeeded, and which earlier paths were skipped or failed.
- Newly generated files, especially `.slipway.yaml` and any selected adapter directories such as `.claude/skills`, `.codex/skills`, `.github/skills`, `.cursor/skills`, `.kilocode/skills`, `.kiro/skills`, `.opencode/skills`, `.pi/skills`, `.qwen/skills`, or `.windsurf/skills`.
- Any unresolved follow-ups the user should know about (for example, a missing release on this platform or `slipway init` choices that still need a human decision).

For OpenCode specifically, the expected generated project surfaces are:

- `.opencode/skills/slipway-*/SKILL.md`
- `.opencode/commands/slipway-*.md`
- `.opencode/hooks/slipway-session-start`
- `.opencode/hooks/slipway-session-start.ps1`
- `.opencode/hooks/slipway-session-start.cmd`

OpenCode commands use slash-hyphen spelling such as `/slipway-new`, `/slipway-next`, and `/slipway-run`. Some OpenCode builds display project commands with a project prefix in the command picker; the generated file path is the stable contract.

Adapters that use generated hook launchers, including Cursor and OpenCode,
receive native launcher files for POSIX, PowerShell, and `cmd.exe` under their
`hooks/` directory. Settings-capable hook hosts (Claude and Qwen)
instead register bare inline `slipway hook ...` commands directly in
`settings.json` and get no launcher file. Pi settings register skills and
prompts, not hooks. Either way, no generated hook requires bash, Python, `jq`,
`gh`, or a Go runtime.

Generated skill helpers run through `slipway tool ...` rather than generated
script payloads. Manual helpers may still require explicit authenticated
backends or domain tools, such as `gh` for GitHub helpers or `go` for Go test
pollution tracing, and fail closed with remediation when those are unavailable.

## Verify Installation

```bash
slipway --version
slipway status --json
git status --short --branch
```

In a repository initialized with adapters, inspect generated files:

```bash
find .claude .codex .github/skills .github/prompts .github/copilot .cursor .kilocode .kiro .opencode .pi .qwen .windsurf -maxdepth 3 -type f 2>/dev/null
```

Codex command surfaces are generated as skills under
`.codex/skills/slipway-<command>/SKILL.md`. Codex refresh only manages the
project-local `.codex/` adapter tree; it does not touch host-global
`$CODEX_HOME/prompts/` or `~/.codex/prompts/` files. For hook-capable adapters,
`--refresh` removes Slipway-owned retired hook launchers. Settings-capable
hosts migrate retired launcher-path settings entries to bare inline
`slipway hook ...` commands; Cursor and OpenCode keep their file-by-path
session-start launchers.
