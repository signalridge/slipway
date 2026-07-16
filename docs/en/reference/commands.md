# Command reference

Slipway exposes seven public commands. Run `slipway <command> --help` against the binary you are using; package channels can contain an older command generation.

| Command | Purpose |
| --- | --- |
| `install` | Generate capabilities for selected AI coding hosts. |
| `uninstall` | Remove pristine Slipway-managed host files. |
| `list` | Show adapter detection and installation state. |
| `doctor` | Diagnose repository, adapter, GitHub-tooling, and Run-storage conditions. |
| `run` | Start an ad-hoc or issue-backed Run and return its first Action. |
| `status` | List Runs or inspect one Run. |
| `stop` | Stop a Run without deleting recovery data. |

All commands accept `--help`. JSON-producing commands include `contract_version`; machine consumers must validate the documented version rather than parsing human prose.

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--surface ide|cli] [--refresh] [--json]
```

Without `--tool`, selects detected hosts. Repeat `--tool` to select several hosts. A first Kiro installation requires exactly one `--surface`. In a mixed selection, `--surface` applies only to Kiro; it is invalid only when Kiro is not selected. `--tool all --surface ide` and `--tool all --surface cli` are valid.

A new install claims only files it creates. `--refresh` updates matching Slipway-owned files and recreates missing pristine files. Modified or unknown content is preserved or reported rather than overwritten.

JSON reports selected hosts, transaction outcome, written and removed paths, preserved content, recovery artifacts, and warnings. A non-committed transaction does not claim planned writes or removals as completed.

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

Removes only hash-matching managed files. Modified files and host settings remain. Run journals are not removed.
Without `--tool`, selects every host that has an ownership manifest and fails if none are installed. Repeating `--tool` limits removal to the named hosts.

## `slipway list`

```text
slipway list [--root PATH] [--json]
```

Lists all ten adapter targets with detection, installation, refresh, and capability information. A malformed or unsupported ownership manifest degrades that host's read-only result without changing files or hiding the other hosts.

## `slipway doctor`

```text
slipway doctor [--root PATH] [--json]
```

Checks repository discovery, host adapters, generated files, Run-storage durability, GitHub CLI/authentication/repository permissions, and retired-state residue. Advisory GitHub or residue findings do not mutate a Run. Authentication responses and tokens are never copied into the report.

`doctor` describes what it observed; it does not run project tests or decide whether code is ready.

## `slipway run`

```text
slipway run [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json] -- <goal>
```

Creates a Run and returns its initial `orient` Action. The Action budget defaults to 8 and must be between 1 and 1000. `--no-review` disables advisory Review; otherwise Review is issued only after an Action for which Slipway observes code changes.

Without `--source-file`, the Run is ad hoc. With it, the CLI opens and validates one bounded GitHub Change source envelope, pins accepted sections, and closes the file. The CLI does not fetch GitHub or show host publication warnings; generated host instructions perform those host-side steps.

Canonical machine invocation puts all flags before `--` and the goal after it:

```bash
slipway run --budget 8 --json --root /absolute/repository -- "small private fix"
slipway run --budget 8 --json --root /absolute/repository \
  --source-file /private/temp/change-envelope.json -- "implement the Change"
```

The command returns an Action; it does not execute the requested code change.

## `slipway status`

```text
slipway status [run-id] [--root ROOT] [--json]
```

Without an ID, lists Runs in the repository's Git common directory. Current-worktree Runs are replayed; another linked worktree's Run appears only as a read-only header marked `workspace_foreign`. Full inspection and mutation require the owning worktree.

`status` is filesystem-read-only: it does not create the run namespace or lock files, change permissions, or repair an interrupted journal tail. A local recovery directory that cannot be replayed remains visible in JSON under `unavailable_runs`; targeted inspection reports `run_journal_invalid`, while an absent ID reports `run_not_found`. If a writer holds the commit boundary through the bounded inspection timeout, targeted and list output report `run_busy` instead of misclassifying the journal as invalid.

With an ID, returns the current Run projection and a freshly derived structured `next` operation. Empty list output is valid.

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

Stops a Run and preserves its journal. Omitting the ID scans listed active or paused entries and proceeds only when that count is one; any unreadable local recovery directory also requires an explicit ID rather than being ignored. An active or paused `workspace_foreign` stub is not selected implicitly. A stopped Run can resume; an ended Run cannot.

## Hidden host operations

Generated adapters use versioned `_machine` operations to submit an Outcome, answer or skip an Action, resume a Run, and read pinned material. They are intentionally absent from top-level help and are not a second user workflow.

Use the structured `next` variants returned by the CLI rather than constructing hidden commands from prose. See the [machine protocol](machine-protocol.md).
