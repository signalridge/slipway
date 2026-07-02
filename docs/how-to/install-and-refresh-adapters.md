# How To Install And Refresh Adapters

Use this guide when you need a working Slipway binary and generated AI-tool
surfaces for the current repository.

For the complete release matrix, checksums, container image, package-manager
channels, and source-build details, see [Installation](../installation.md).

## Install The CLI

Use a release-backed channel when possible:

| Platform | Recommended path |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | Use the `.deb`, `.rpm`, `.apk`, `tar.gz`, AUR, or container image paths in [Installation](../installation.md#linux). |

Use Go install when release packages are unavailable or you intentionally want a
Go-managed binary:

```bash
go install github.com/signalridge/slipway@latest
```

Verify the install:

```bash
slipway --help
```

If an AI tool finds a same-name package in an unrecognized registry, stop and
verify ownership before installing it.

## Initialize Slipway In A Repo

From the repository root:

```bash
slipway init --tools codex
```

Common adapter choices:

```bash
slipway init --tools claude
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`--tools none` initializes the runtime layout and `.slipway.yaml` without
writing host adapter files.

Inspect the diff before committing generated files:

```bash
git status --short
git diff -- .slipway.yaml .claude .codex .cursor .opencode
```

Commit `.slipway.yaml` when the repo should share Slipway defaults. Commit
generated adapter files only according to the repository's policy.

## Refresh Existing Adapters

Refresh auto-detected Slipway-managed adapters:

```bash
slipway init --refresh
```

Refresh a specific set:

```bash
slipway init --tools codex,opencode --refresh
```

Refresh every supported adapter:

```bash
slipway init --tools all --refresh
```

Refresh detects Slipway generated markers. It does not treat a bare `.claude`,
`.codex`, `.cursor`, or `.opencode` directory as owned by Slipway.

## Preserve User-Owned Files

Before accepting a refresh diff, check adjacent host config:

```bash
git status --short .claude .codex .cursor .opencode
```

Generated files route to the CLI. User-owned host settings, local prompts,
manual commands, and non-Slipway hooks should stay intact.

If refresh output removes a legacy Slipway-owned launcher or prompt, verify that
the new generated surface exists before committing. Codex command surfaces now
live under:

```text
.codex/skills/slipway-<command>/SKILL.md
```

## After Changing Command Or Skill Surfaces

If you changed command registrations, generated skills, JSON contracts, or docs
tokens, update the surface manifest:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go test ./internal/toolgen -run SurfaceManifest -count=1
```

The manifest is derived from Go authorities and documentation tokens. Do not
hand-edit generated rows unless you are repairing the generator itself.

## Related

- [AI tool adapters](../reference/ai-tools.md)
- [Commands](../reference/commands.md)
- [Recover and troubleshoot](recover-and-troubleshoot.md)
