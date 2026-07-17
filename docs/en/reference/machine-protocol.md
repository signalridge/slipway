# Machine protocol

This page is for adapter and integration authors. Regular users should invoke a generated host capability and use the [command reference](commands.md).

The current JSON contract version is **2**:

- [machine-protocol.schema.json](../../reference/v2/machine-protocol.schema.json) defines public command, Action, Outcome, status, error, and recovery shapes.
- [source-envelope.schema.json](../../reference/v2/source-envelope.schema.json) defines the GitHub source transport shape.

Use the [machine protocol v2 tutorial](../guides/machine-protocol-v2.md) for a runnable start-to-end host exchange.

The schemas define serialization shape. Runtime validation also enforces rules that JSON Schema cannot express completely: embedded manifest syntax, ordering, hashes, cross-field identity, idempotency, workspace state, and filesystem safety. Validate against the schema and preserve CLI errors rather than reimplementing the Go validator from prose.

## Process boundary

The host calls the model, reads the repository, runs tools, and—when requested—uses GitHub credentials. Within this protocol exchange, the Run/source path is local and deterministic: it validates messages, records the Run, observes Git, and returns the next operation without calling a model or accessing GitHub. The separate public `doctor` command may invoke the user's local `gh` for read-only diagnostics.

A host normally uses JSON on every step:

```text
slipway run --budget N --json --root ROOT [--no-review] [--source-file FILE] -- GOAL
slipway protocol submit --run RUN --action ACTION --root ROOT (--outcome-file FILE | --outcome-stdin)
slipway protocol answer --run RUN --action ACTION --root ROOT --text TEXT
slipway protocol answer --run RUN --action ACTION --root ROOT --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway protocol skip --run RUN --action ACTION --root ROOT
slipway protocol resume RUN --root ROOT [--budget N]
slipway protocol resume RUN --root ROOT (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway protocol material --run RUN --action ACTION --root ROOT --section KEY
```

The hidden operations are versioned host interfaces. Do not expose them as an alternative end-user command sequence.

## Start a Run

Canonical invocation places all flags before one `--` separator and the literal goal after it:

```bash
slipway run --budget 8 --json --root /absolute/worktree -- "one goal"
```

An ad-hoc Run omits source fields. An issue-backed Run supplies a private temporary `--source-file`; the CLI consumes it once and does not depend on the file or GitHub afterward.

The start response contains the Run state, the initial `orient` Action, and a structured `next` operation.

## Action

An active Run contains one non-null Action:

```json
{
  "contract_version": 2,
  "run_id": "...",
  "action_id": "...",
  "kind": "orient",
  "goal": "...",
  "brief": "...",
  "context": "...",
  "remaining_budget": 7
}
```

`kind` is one of `orient`, `clarify`, `implement`, `review`, or `summarize`.

Issue-backed Actions additionally contain:

- source, manifest, and requirements revisions;
- an ordered, bounded section catalog;
- a structured `protocol material` reader;
- the section keys required for the current Action.

They do not copy requirements Markdown into `context`. The material reader is valid only for the current non-void Action and verifies digest, byte count, and section revision before returning content.

The versioned field for those keys is `requirements.required_for_action`. In protocol v2 it is the ordered list of every key in `requirements.sections`; hosts must preserve that exact equality rather than infer a smaller subset.

`context` is a bounded projection of active answers and prior Outcome summaries. It is not the full journal, source, conversation, or hidden model reasoning.

## Outcome

Submit an Outcome from exactly one input:

```text
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-file FILE
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-stdin
```

Every public Outcome field is present. Arrays remain arrays when empty; inapplicable object branches are JSON `null`:

```json
{
  "contract_version": 2,
  "action_id": "...",
  "action_kind": "orient",
  "status": "completed",
  "summary": "observed facts",
  "observations": [],
  "known_issues": [],
  "suggested_actions": [],
  "pause": null,
  "implementation": null,
  "review": null
}
```

`action_kind` must match the outstanding Action. Host status is `completed`, `needs_input`, `partial`, or `error`; skip is a CLI operation, not an Outcome status.

- Orient or Clarify may suggest at most one immediate `clarify`, `implement`, or `summarize` Action.
- A non-paused Implement uses the `implementation` branch and reports actual files, attempts, uncertainty, and test/type-check/build/lint activities with exit codes.
- A non-paused Review uses the `review` branch and reports findings; it does not suggest repair work.
- Summary and every `needs_input` Outcome have no suggested Action.

### Legal Outcome combinations

| Action | Host status | Required result branches | Allowed pause | Allowed suggestions |
| --- | --- | --- | --- | --- |
| Orient | `completed` / `partial` / `error` | `implementation=null`, `review=null` | none | zero or one Clarify, Implement, or Summarize |
| Orient | `needs_input` | `implementation=null`, `review=null` | decision or environment | none |
| Clarify | `completed` / `error` | `implementation=null`, `review=null` | none | zero or one Clarify, Implement, or Summarize |
| Clarify | `needs_input` | `implementation=null`, `review=null` | decision or environment | none |
| Implement | `completed` | `implementation.result=applied\|not_needed`, `review=null` | none | none |
| Implement | `partial` | `implementation.result=partial`, `review=null` | none | none |
| Implement | `error` | `implementation.result=unable`, `review=null` | none | none |
| Implement | `needs_input` | `implementation=null`, `review=null` | decision, destructive, or environment | none |
| Review | `completed` | `review.result=no_findings_reported\|findings_reported`, `implementation=null` | none | none |
| Review | `partial` | `review.result=inconclusive`, `implementation=null` | none | none |
| Review | `error` | `review.result=error`, `implementation=null` | none | none |
| Summarize | `completed` / `error` | `implementation=null`, `review=null` | none | none |

Clarify intentionally has no legal `partial` combination: it carries one decision at a time. Review cannot use `needs_input` or suggest Implement, and `not_run` is a CLI-owned review-skip projection.

A `needs_input` Outcome has one pause reason: `decision_required`, `destructive_confirmation_required`, or `environment_unavailable`. `budget_exhausted` is produced only by the CLI.

Destructive confirmation is valid only for an exact current Implement request and scope digest. Natural-language text such as “yes” is feedback, not authorization. Any Action change, resume, scope expansion, or mismatch invalidates the grant.

Outcome input is limited to 1 MiB, valid UTF-8, with no BOM, duplicate fields, unknown fields, or trailing data.

## Structured `next`

Every success or error that can continue contains a typed `next` object. It has:

- `operation`: `action`, `answer`, `resume`, `start`, `command`, or `none`;
- the original `workspace_identity`;
- zero or more variants with `id`, `base_argv`, and typed inputs.

Input types are `string`, `path`, `enum`, and `digest`. A consumer selects one variant and inserts supplied values as separate argv elements in schema order. It must not parse or concatenate a display command.

Only a fully resolved, inputless variant may be rendered for a human shell. POSIX, `cmd.exe`, and PowerShell rendering is presentation only; structured argv is the machine value.

When a Windows display command contains expansion-sensitive `%` or `!` values that `cmd.exe` cannot preserve safely, the renderer uses a PowerShell UTF-16LE `EncodedCommand` trampoline. This changes only the copyable display form; the decoded process argv must remain byte-for-byte equivalent to the structured variant.

Ended Runs use `operation: "none"` with an empty variant list.

## Source envelope

A source envelope is at most 16 MiB and identifies one `github.com` Issue by repository and Issue node IDs. For a valid Change:

- the first nonempty body line is `<!-- slipway-level: change/v2 -->`;
- the next nonempty block is one strict `slipway-manifest` JSON fence;
- the ordered manifest has 5–64 section entries and includes outcome, requirements, acceptance examples, constraints, and non-goals roles;
- the envelope contains exactly the referenced comments;
- each comment starts with its exact section marker and matches its declared digest.

A normalized section is at most 256 KiB; the complete section payload is at most 4 MiB; the manifest is at most 256 KiB. Missing, extra, duplicate, minimized, edited, oversized, or hash-mismatched references are rejected.

The top-level source schema intentionally permits empty or whitespace-only Issue/comment bodies and an empty comments array for an invalid refreshed head. This lets the CLI classify missing markers, empty referenced sections, and digest mismatches without the host rejecting the envelope or collecting unrelated discussion. The embedded manifest string and all semantic digest checks are validated by the runtime, not by the top-level schema alone.

The CLI persists stable identity, provenance, byte counts, revisions, and content-addressed accepted material. It does not journal the raw envelope, title labels as requirements, source-file path, or unreferenced comments.

## Refresh and candidates

An issue-backed resume explicitly does one of the following:

- imports and compares a fresh envelope;
- continues from the pinned snapshot;
- resolves the exact current candidate by keeping pinned content or adopting a valid candidate.

No source option means neither “unchanged” nor implicit network access. A different Issue identity and an amendment based on another parent requirements revision are rejected without changing the Run. Candidate IDs and choices are stale-safe and idempotent.

A successful resume voids stale outstanding work as required, revalidates the workspace, and normally returns a fresh Orient. If `--budget` is omitted, a positive remaining budget is preserved and zero is replenished to `max(initial_budget, 3)`; an explicit `--budget N` replaces it with `N`. A replacement is applied only on the mutation that actually resumes the Run.

## Workspace and Git observation

Workspace identity includes the canonical worktree root, per-worktree Git directory, and Git common directory. Every load or mutation rediscovers and compares those paths before changing a journal.

Repository-wide `status` is the filesystem-read-only exception: it creates no namespace or lock, changes no permissions, and repairs no journal bytes. Runs from another linked worktree appear as FirstEvent header stubs with `workspace_foreign` and are not fully replayed outside their owning worktree. Unreadable local Run directories remain identified in list JSON as `unavailable_runs`; targeted corruption and absence are distinct errors.

Git observations record hashes and bounded metadata for the index, porcelain status, and dirty paths. They never retain file content. A difference is evidence of change since Run start, not proof that the current host caused it.

## Idempotency and ordering

- Outcome idempotency hashes the exact accepted input bytes; differently serialized JSON conflicts even if semantically similar.
- Answer, skip, resume, and candidate operations bind to current IDs and reject stale or conflicting retries.
- Each Run has one writer at a time, enforced by the platform lock implementation.
- Journal order is the recovery record; `run.json` may be rebuilt.

## Errors and compatibility

Machine errors include `contract_version`, a stable `code`, human `message`, `exit_code`, and structured recovery where available. Preserve all fields and branch on code/version, not message text.

`journal_record_too_large` carries the strict detail fields `context`, `size`, and `limit`, plus a read-only `status` recovery variant for the affected Run when its ID is known. Rejecting an oversized record does not end or invalidate the persistent Run.

Unknown contract versions and fields are rejected. Version 2 does not promise compatibility with unreleased development formats that preceded it. A future incompatible contract must use a new explicit version rather than a silent alias.
