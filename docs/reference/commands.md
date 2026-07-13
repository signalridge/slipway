# Command reference

Slipway exposes exactly seven public commands.

This English reference is non-normative. The [Chinese product contract](../zh/reference/product-contract.md) and [machine schema](machine-protocol.schema.json) are authoritative. Slipway is issue-first, not issue-gated; see the [Issue workflow](issue-workflow.md).

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--refresh] [--json]
```

Installs six explicit host capabilities. Without `--tool`, uses detected hosts. `--tool all` selects every supported host. With `--json`, install returns exactly `{contract_version,hosts,written,removed,preserved,warnings}`; every list is present even when empty.
A first install does not require `--refresh`. If a current ownership manifest already exists, plain `install` leaves it unchanged; use `--refresh` to repair missing pristine files or update owned capabilities. A marker without a current manifest produces a no-op safety warning. Any non-current manifest is unreadable and the command fails without mutation.

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

Removes only hash-matching managed files. Modified files are preserved and reported. Its `--json` response uses the same exact versioned change-report envelope as install.

## `slipway list`

Lists every adapter with `detected`, `installed`, `needs_refresh`, and capability information. An incomplete current managed surface is not reported as healthy; any non-current manifest makes list fail closed. JSON is exactly `{contract_version,hosts:[{id,detected,installed,needs_refresh,capabilities}]}`; an empty result is `{"contract_version":2,"hosts":[]}`.

## `slipway doctor`

Diagnoses Git repository discovery, manifests, generated files, host installation state, GitHub CLI/auth/repository permissions, and legacy runstore residue. JSON is exactly `{contract_version,checks:[{code,status,host_id,name,detail}]}`; every check has all five fields and `status` is `ok|warning|error`.

Repository/adapter codes are `repository_ok`, `adapter_manifest_unreadable`, `adapter_not_detected`, `adapter_not_installed`, `adapter_refresh_required`, `adapter_modified`, and `adapter_healthy`.

GitHub codes are `github_cli_unavailable`, `github_cli_version_unknown`, `github_cli_rest_fallback_required`, `github_cli_compatible`, `github_auth_unavailable`, `github_auth_available`, `github_issue_permissions_ok`, `github_issue_permissions_limited`, and `github_issue_permissions_unknown`. `gh` older than 2.94.0 requires the official REST fallback for parent/sub-issue/dependency operations. Authentication and API output are never copied into the report.

Legacy codes are `legacy_runtime_residue`, `legacy_cache_residue`, `legacy_scope_root_residue`, `legacy_scopes_residue`, `legacy_locks_residue`, `legacy_processes_residue`, `legacy_repair_backups_residue`, and `legacy_unknown_residue`. Doctor reads only top-level metadata, never migrates or deletes residue, and advises stopping the old binary, backing up, then cleaning manually if desired. GitHub and legacy warnings are advisory: they do not mutate a Run or make an ad-hoc Run unhealthy, and doctor exits successfully when they are the only findings.

## `slipway run`

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json]
```

Creates a journal and returns the initial `orient` Action. The default Action budget is 8. `--no-review` omits the recommended review after observed code changes. Without `--source-file`, the Run is ad-hoc. With `--source-file`, Slipway opens one strict Source Bundle v2 envelope, verifies the Issue-body manifest and its exact referenced comments, durably stores each normalized chapter by digest, and journals only the bounded catalog. The temporary file and GitHub are never needed for local material reads or resume.

Ad-hoc and issue-bound examples:

```bash
slipway run "small private fix" --json
slipway run "implement the bounded Change" --source-file /safe/temp/change-envelope.json --json
```

Before source import, warn that accepted Requirements, goals, later answers, and command summaries may be sensitive. Raw Issue/comment bodies, tokens, labels, and the source-file path are not journal inputs; accepted chapter bytes live only in private per-Run material blobs. See [Runs and privacy](../explanation/runs-and-privacy.md).

Host-only protocol commands are intentionally hidden from help:

```text
slipway _machine submit --run ID --action ID (--outcome-file FILE | --outcome-stdin)
slipway _machine answer --run ID --action ID --text TEXT
slipway _machine answer --run ID --action ID --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway _machine skip --run ID --action ID
slipway _machine resume ID [--budget N]
slipway _machine resume ID (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway _machine material --run ID --action ID --section KEY
```

The first resume form is ad-hoc only. Issue-bound resume requires exactly one source mode: import a fresh envelope, explicitly continue with the pinned snapshot, or resolve the current candidate by its exact ID. See [Machine protocol](machine-protocol.md).

## `slipway status`

```text
slipway status [run-id] [--root ROOT] [--json]
```

Without an ID, lists all journals in the Git common directory, including runs from linked worktrees. JSON list output is exactly `{contract_version,runs:[...]}` and an empty list is `{"contract_version":2,"runs":[]}`. With an ID, JSON remains the documented flat Run status projection: its mandatory top-level `contract_version` and freshly derived `next` sit beside the Run fields.

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

Stops without deleting recovery data. An omitted ID is accepted only when exactly one active or paused run exists. A stopped run can be resumed.

All protocol errors are JSON and include `contract_version`, `code`, `message`, structured `next`, and `exit_code`. Every `next` variant preserves the original absolute `--root`; only complete inputless/resolved argv is rendered for humans.
