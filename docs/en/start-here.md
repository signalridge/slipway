# Start here

This guide takes a current Slipway build from an empty checkout to one user-controlled Run.

## 1. Confirm the CLI generation

The interface documented here has seven user commands plus the machine protocol:

```bash
./slipway --help
```

The output must list `install`, `uninstall`, `list`, `doctor`, `run`, `status`, and `stop`, alongside the `protocol` group that generated adapters call. If it does not, the installed package belongs to an earlier release. Build this checkout or choose a newer compatible tag; see [Installation](installation.md).

## 2. Install one host adapter

Run inside the Git worktree where the AI host will work:

```bash
./slipway install --tool claude
./slipway doctor
```

Replace `claude` with `codex`, `copilot`, `cursor`, `kilo`, `opencode`, `pi`, `qwen`, or `windsurf`. Kiro needs a surface on first install:

```bash
./slipway install --tool kiro --surface ide   # or: --surface cli
```

`install` writes only host-local capability files and records their hashes. It does not change global host settings or install ambient hooks. See [Host adapters](reference/adapters.md) for generated paths.

## 3. Start a Run explicitly

In the AI coding host, explicitly invoke the generated `slipway-run` capability and give it one task, for example:

> Add a CSV export to the reports command and cover it with tests.

The host asks the CLI for an Action, performs that Action, reports a structured Outcome, and repeats until the Run pauses or reaches its summary. The `protocol` operations it uses are documented, but you do not need to drive them manually: each response already carries the exact next command.

A direct CLI integration can start the same ad-hoc Run with:

```bash
./slipway run --json -- "add CSV export to the reports command"
```

This command returns the first `orient` Action. The CLI does not edit code by itself.

## 4. Choose a source

| Source | Choose it when |
| --- | --- |
| Ad hoc | The task is small, private, urgent, offline, or does not need an Issue. |
| GitHub Change Issue | The requirements need a durable, reviewable source that a Run pins by revision. |

For an issue-backed Run, use the generated `slipway-run` capability with a GitHub Change Issue. The host fetches the Issue, builds the temporary source envelope, and passes it to the CLI. Do not hand-author the envelope unless you are implementing a host integration.

An Objective can group several Changes, but it cannot start a Run. Read [Using GitHub Issues](guides/github-issues.md) before publishing managed Issues.

## 5. Stay in control

A Run may pause for one of five reasons:

- a genuine human decision;
- a changed or unavailable issue source;
- an unavailable environment dependency;
- exhaustion of the Action budget;
- confirmation of an exact destructive scope.

The generated host presents the available response. You may also skip, stop, reorder, or take over work without giving a reason. Ordinary implementation does not ask for repeated authorization.

Useful inspection commands are:

```bash
./slipway status
./slipway status <run-id> --json
./slipway stop <run-id>
```

`stop` preserves recovery data. An ended Run means only that Slipway has no more automatic Actions; tests, review findings, repository policy, merge approval, and release decisions remain separate facts.

## 6. Know what is stored

Run data is under `<git-common-dir>/slipway/runs/`. It may contain the goal, accepted requirements, user answers, Outcomes, and command summaries. Slipway does not intentionally collect tokens, environment dumps, unrelated files, full conversations, or hidden reasoning, but it cannot guarantee a secret-free journal.

Read [Runs, recovery, and privacy](guides/runs-and-recovery.md) before using sensitive content.

## Next

- [Core concepts](explanation/concepts.md)
- [Command reference](reference/commands.md)
- [Host adapters](reference/adapters.md)
- [Machine protocol](reference/machine-protocol.md)
