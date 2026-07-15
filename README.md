<div align="center">

<img alt="Slipway" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>
</p>

[Documentation](https://signalridge.github.io/slipway/en/) ·
[Start here](docs/en/start-here.md) ·
[Installation](docs/en/installation.md) ·
[Release notes](CHANGELOG.md)

**English** · [简体中文](README.zh.md) · [日本語](README.ja.md)

</div>

# Slipway

Slipway is a user-invoked soft autopilot for AI coding. It gives an AI coding host a
small, recoverable workflow while keeping decisions and control with the user.

A Slipway Run moves through one bounded Action at a time:

```text
orient → clarify when needed → implement → review when enabled and code changed → summarize
```

The host performs the work. The Slipway CLI records the Run, chooses the next Action,
observes repository changes, and provides structured recovery. It does not call a model,
hold a GitHub token, or decide that the software is ready to merge or release.

> [!IMPORTANT]
> Use a build whose `slipway --help` lists `install`, `uninstall`, `list`, `doctor`,
> `run`, `status`, and `stop`. Package-manager channels can lag the repository. Until a
> tagged release contains this interface, build the current checkout from source.

## Why use Slipway?

- **Explicit start:** nothing runs ambiently. You invoke a generated capability or the
  CLI for a specific task.
- **User control:** skip, stop, resume, reorder, or take over without explaining why.
- **Facts before questions:** the host inspects the repository before asking for a
  genuine product decision.
- **Recoverable Runs:** an append-only journal and pinned source material let a Run
  resume without relying on chat history.
- **Optional GitHub source:** use a self-contained Change Issue for durable work, or
  start ad hoc when an Issue is unnecessary, unavailable, or inappropriate.
- **Honest outcomes:** commands, exit results, findings, known issues, and uncertainty
  are reported. An ended Run means only that its Action queue is empty.

## Quick start from this checkout

Build Slipway with the Go version declared in [`go.mod`](go.mod), then install the
adapter for your AI coding host:

```bash
go build -o ./slipway .
./slipway install --tool claude
./slipway doctor
```

In Claude, explicitly invoke the generated `slipway-run` skill and describe one task.
For another host, replace `claude` with one of:

```text
codex  copilot  cursor  kilo  kiro  opencode  pi  qwen  windsurf
```

Kiro requires a surface on first install:

```bash
./slipway install --tool kiro --surface ide   # or: --surface cli
```

The generated capability drives the machine protocol for you. If you are integrating a
host directly, an ad-hoc Run starts with:

```bash
./slipway run --json -- "add CSV export to reports"
```

That command returns the first Action; it does not itself edit code. See the
[getting-started guide](docs/en/start-here.md) for the complete interaction and the
[machine protocol](docs/en/reference/machine-protocol.md) for integration details.

## Sources and Runs

| Source | Use it when | What Slipway records |
| --- | --- | --- |
| Ad hoc | The task is small, private, urgent, offline, or deliberately not tracked in GitHub. | The goal and subsequent Run events. |
| GitHub Change Issue | The task needs a reviewable, revision-pinned requirements source. | Stable Issue identity, a bounded section catalog, and accepted section material by digest. |

A GitHub Objective Issue may group several Changes, but only a self-contained Change can
start an issue-backed Run. Repository ownership by a personal account or an organization
is not part of the source format. Slipway does not require GitHub Projects,
organization-only Issue Types, or organization-only fields.

Generated `propose` and `decompose` capabilities can help prepare Issues. Those are
host-side operations: the host previews external writes, uses the user's GitHub access,
and reports partial or failed publication. The Run/source core neither fetches nor
publishes GitHub data and does not store credentials. The separate `doctor` command may
invoke the user's local `gh` for read-only diagnostics.

## Control and recovery

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

When a Run needs input, the generated host presents the exact decision, source choice,
environment problem, or destructive scope. Work continues only after the corresponding
explicit response. `stop` keeps recovery data; deleting a Run directory removes local
recovery but is not secure erasure.

Run data lives under the repository's Git common directory at
`<git-common-dir>/slipway/runs/`. It can contain goals, accepted requirements, answers,
and command summaries. Treat it as private local data; Slipway minimizes collection but
does not promise that a journal is secret-free.

Read [Runs, recovery, and privacy](docs/en/guides/runs-and-recovery.md) before using
sensitive material.

## Documentation

### Use Slipway

- [Start here](docs/en/start-here.md)
- [Installation](docs/en/installation.md)
- [GitHub Issue workflow](docs/en/guides/github-issues.md)
- [Runs, recovery, and privacy](docs/en/guides/runs-and-recovery.md)
- [Core concepts](docs/en/explanation/concepts.md)

### Look up exact surfaces

- [Command reference](docs/en/reference/commands.md)
- [Host adapters](docs/en/reference/adapters.md)
- [Machine protocol](docs/en/reference/machine-protocol.md)
- [Architecture](docs/en/explanation/architecture.md)

### Contribute

- [Contributing](CONTRIBUTING.md)
- [Development reference](docs/en/contributing.md)
- [Acceptance suite](acceptance/README.md)
- [Architecture decisions](adr/README.md)

## License

Slipway is distributed under the [BSD 3-Clause License](LICENSE).
