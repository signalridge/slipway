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

AI coding tools are fast, but a long autonomous session can stray from the task you
asked for, claim to have run checks it skipped, or declare the work done before the
changes land in the repository.

Slipway is a user-invoked soft autopilot. It wraps an AI coding session in a
lightweight, resumable workflow while keeping decisions and control with the user.

A Slipway Run moves through one bounded Action at a time: `orient`, `clarify`,
`implement`, `review`, or `summarize`. The order is not a fixed pipeline — the CLI
derives each Action from the last Outcome and its own Git observation.

The host performs the work. The Slipway CLI records the Run, observes repository
changes, and provides structured recovery. It does not call a model, hold a GitHub
token, or decide that the software is ready to merge or release.

![Slipway Run lifecycle: an explicit start enters a one-Action-at-a-time loop in which the CLI issues an Action, the host performs it and returns a structured Outcome, and the CLI validates, records, and observes Git before choosing what happens next. The user can skip without a reason, stop, or resume. Ended means only that the automatic Action queue is empty, not that the work is correct, merged, deployed, or ready to release.](docs/assets/diagrams/lifecycle.svg)

> [!IMPORTANT]
> Use a build whose `slipway --help` lists `install`, `uninstall`, `list`, `doctor`,
> `run`, `status`, and `stop`, alongside the `protocol` group that generated adapters
> call. Package-manager channels can lag the repository. Until a tagged release
> contains this interface, build the current checkout from source.

## Why use Slipway?

- **Explicit start:** nothing starts on its own. You invoke a generated capability or
  the CLI for a specific task.
- **User control:** skip, stop, resume, reorder, or take over. No explanation needed.
- **Facts before questions:** the host inspects the repository before asking you to
  make a decision.
- **Recoverable Runs:** an append-only journal and pinned source material let a Run
  resume without relying on chat history.
- **Optional GitHub source:** use a self-contained Change Issue for durable work, or
  start ad hoc when an Issue is unnecessary, unavailable, or inappropriate.
- **Honest outcomes:** every command, exit code, finding, known issue, and uncertainty
  is reported. An ended Run only means the Action queue is empty.

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
./slipway run -- "add CSV export to the reports command"
```

```text
Run d51ecc3e-3b77-4e27-b9e4-f32fa5a0f11c started.
State: active
Goal: add CSV export to the reports command
Budget remaining: 7
Current action: orient (6da7c41d-7283-4c68-9238-868f3502f67c)
Next choices:
- submit-outcome-file: requires outcome_file (path via --outcome-file)
- submit-outcome-stdin: slipway protocol submit --run d51ecc3e-… --action 6da7c41d-… --root /path/to/reports --outcome-stdin
- skip-action: slipway protocol skip --run d51ecc3e-… --action 6da7c41d-… --root /path/to/reports
```

That is the shape of a Run: one Action at a time, a remaining budget, and an exact
next command for every branch the host can take — including skip, which never needs a
reason. The CLI chose and recorded that Action; it never edits code on its own.
Add `--json` for the machine-readable form.

See the [getting-started guide](docs/en/start-here.md) for the complete interaction and
the [machine protocol](docs/en/reference/machine-protocol.md) for integration details.

## Sources and Runs

| Source | Use it when | What Slipway records |
| --- | --- | --- |
| Ad hoc | The task is small, private, urgent, offline, or deliberately not tracked in GitHub. | The goal and subsequent Run events. |
| GitHub Change Issue | The task needs a reviewable, revision-pinned requirements source. | Stable Issue identity, a bounded section catalog, and accepted section material by digest. |

A GitHub Objective Issue may group several Changes, but only a self-contained Change can
start an issue-backed Run. Repository ownership by a personal account or an organization
is not part of the source format. Slipway does not require GitHub Projects,
organization-only Issue Types, or organization-only fields.

The generated `workflow` capability coordinates the functions in Slipway's Issue
workflow. From a rough idea, Objective, Change, or existing Run it inspects the current
stage and names the shortest valid next explicit capability or an explicit
no-further-action outcome. It may investigate facts, interview genuine decisions, and
synthesize a work-item draft when needed, but it is not a general skill router. It is
self-contained and may reuse only an already-installed, model-invocable `/grilling`
primitive; it never invokes user-only front doors, writes to GitHub, or starts a Run
itself.

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
- [Idea-to-Run workflow](docs/en/guides/idea-to-run-workflow.md)
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
