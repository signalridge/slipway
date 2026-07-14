# Machine protocol

Contract version: **2**. The normative schemas are [machine-protocol.schema.json](machine-protocol.schema.json) and [source-envelope.schema.json](source-envelope.schema.json). The source model is recorded in [ADR-0001](../decisions/0001-source-bundle-v2.md).

The AI coding host executes Actions; Slipway schedules them, observes Git independently, and stores recovery history. Unknown contract versions and unknown or duplicate JSON fields are rejected.

## Start and pin a source

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json
```

`--source-file` is optional for a new Run. When present, it must be a strict Source Bundle v2 envelope no larger than 16 MiB. The Issue body starts with the exact `change/v2` marker followed by one strict `slipway-manifest` JSON fence. Its ordered manifest explicitly binds 5–64 section keys and roles to GitHub comment node IDs and exact body digests. For a valid manifest, the envelope contains exactly those referenced comments; Slipway never scans ordinary discussion comments or treats comment order as authority. A refreshed head without a parseable v2 manifest uses an initialized empty `comments` array so the core can classify the invalid candidate without collecting unrelated discussion.

Each referenced comment starts with `<!-- slipway-section:v1 key=KEY -->`. The normalized chapter after that marker is limited to 256 KiB, and the complete bundle to 4 MiB. Missing, unexpected, minimized, edited, duplicate, or hash-mismatched comments fail closed. Database IDs, URLs, authors, and observation times are provenance; the GraphQL comment node ID is canonical remote identity. The manifest revision commits the node/database-ID binding, while the requirements revision intentionally excludes provenance.

Slipway opens the regular no-follow file once and closes it before parsing. A raw observation contains at most 100 labels; a pinned source retains at most 64 prior transfer URL aliases, after which refresh returns a structured `start-with-source` recovery for a new Run. Before the first journal event that refers to a chapter, its exact normalized bytes are fsynced to the Run's private content-addressed material store. Journals, status, candidates, and Actions retain only catalogs, provenance, byte counts, and domain-separated revisions—not Markdown, the raw Issue body, labels, or source-file path. Replay derives an accepted-comment identity ledger from every pinned manifest head: retiring a comment does not forget its node/database identity, and reintroducing that identity must match the originally accepted section. A Run can therefore resume and read its pinned chapters without GitHub or the temporary source file.

Without `--source-file`, the Run is ad-hoc. Ad-hoc Actions omit both `source` and `requirements`. Issue-bound Actions carry the source, manifest, and requirements revisions plus a bounded chapter catalog and a structured local reader. They never copy chapter Markdown into the Action.

## Resume and source amendments

```text
slipway _machine resume RUN [--budget N]
slipway _machine resume RUN (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
```

The first form normally applies only to ad-hoc Runs and rejects every source option. An issue-bound Run with no current candidate requires exactly one of a freshly imported envelope or `--use-pinned-source`; omission never means “unchanged.” The sole exception is a Run paused with `budget_exhausted` immediately after an exact destructive confirmation: a resume with no source mode only replenishes budget and issues the already-authorized scoped Implement. Any source mode still voids that grant. A Run with a current candidate rejects refresh/pinned modes and requires an exact `--source-choice` plus `--candidate` pair. An invalid candidate permits only `pinned`.

Refresh validates provider/host and issue node ID before any mutation. A different issue is rejected and requires a new Run. Repository, number, or URL transfer updates projection and records the prior canonical URL once while still comparing the marker and requirements. A refresh whose manifest revision is unchanged—including identical, projection-only, and other non-material drift—voids the outstanding Action/queue/authorization and issues a fresh Orient. An invalid body stores a run-local, path-free candidate, voids outstanding work, and pauses with `decision_required` without applying the requested budget. A structurally valid refreshed manifest with a new revision is a candidate only when its declared `parent_requirements_revision` exactly equals the current pinned `requirements_revision`; this includes a content-identical replacement. A different parent means the amendment was authored against another history: Slipway rejects it as `source_history_fork` before creating a candidate or mutating the Run, and the refreshed source requires a new Run.

`pinned` retains the accepted manifest, Requirements, and section content while applying the candidate's same-Issue repository/number/canonical-URL/alias/parent projection; `adopt` installs a valid candidate snapshot. Only when adoption changes `requirements_revision` are answers derived from the old revision removed from active Action context; their records remain. A content-identical manifest-only replacement keeps those answers active. The choice receipt makes an identical `(candidate_id, choice)` retry a no-op; another choice or stale ID conflicts. `--use-pinned-source` records `source_refresh_skipped` and never claims the source was unchanged.

A new Run's Action budget defaults to 8. Any explicit start or resume budget must be 1..1000; a successful resume replacement is applied before the fresh Action consumes one. Normally that Action is Orient; the exact-confirmation exception above issues the scoped Implement directly. If resume omits the budget, a positive remainder is preserved; an exhausted remainder becomes `max(initial_budget, 3)` before the Action. Candidate creation reports `budget_applied: false`; repeat the budget on the subsequent choice. Paused protocol output exposes safe `pinned_source`, `source_candidate`, `resume_operation`, and `budget_applied` fields. Full status JSON also exposes `last_resume_result` and the last choice receipt, but no source-file path.

## Structured `next`

Machine recovery authority is a typed `next` object, never a shell string:

```json
{
  "operation": "resume",
  "workspace_identity": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "variants": [
    {
      "id": "refresh-source",
      "base_argv": ["slipway", "_machine", "resume", "RUN", "--root", "/absolute/original/workspace"],
      "inputs": [
        {"name": "source_file", "type": "path", "flag": "--source-file", "required": true}
      ]
    },
    {
      "id": "use-pinned-source",
      "base_argv": ["slipway", "_machine", "resume", "RUN", "--root", "/absolute/original/workspace", "--use-pinned-source"],
      "inputs": []
    }
  ]
}
```

Input types are `string`, `path`, `enum`, and `digest`; enum inputs list nonempty `choices`, and digest values use lowercase `sha256:<64 hex>`. Resolve a chosen variant by copying `base_argv`, then, in input schema order, insert each supplied `flag` and exact raw value as separate argv elements immediately before the sole `--` separator when present, or append them when no separator exists. This keeps `start` goals—including goals beginning with `-`—as the single literal argv element after `--`. Unknown, missing required, wrong-type, invalid enum, and malformed digest values are rejected. Every variant contains the run's original absolute `--root`; no variant contains `FILE`, `<file>`, `<answer>`, or a quoted pseudo-value.

`next.operation` is one of `action`, `answer`, `resume`, `start`, `command`, and `none`. Runtime validation and the JSON Schema bind every variant to its operation family: `action` uses `_machine submit` (or exact `skip-action`), `answer` uses `_machine answer` (or skip), `resume` uses `_machine resume` (or skip), `start` uses exactly `slipway run --budget N --json --root ROOT [--no-review] -- GOAL`, and `command` may use neither `run` nor `_machine`. Active Actions derive `submit-outcome-file`, inputless `submit-outcome-stdin`, and inputless `skip-action`. Decision pauses derive `answer-decision` plus `skip-action`; destructive pauses derive inputless `confirm-destructive` fixed to the current digest (with optional text), `decline-or-feedback`, and `skip-action`; environment pauses derive the appropriate resume variants plus `skip-action`. Budget/candidate pauses have no waiting Action and therefore do not expose skip. Ad-hoc recovery derives `resume-ad-hoc`; issue recovery derives `refresh-source` and `use-pinned-source`; a budget-blocked exact destructive confirmation derives only `replenish-destructive-budget`; valid candidates derive `keep-pinned` and `adopt`, while invalid candidates expose only `keep-pinned`. Ended Runs use operation `none` and an empty variants array.

Only a variant with no unresolved required input may be rendered as a display command. Rendering happens at the CLI edge from argv for POSIX, `cmd.exe`, or PowerShell and never changes machine semantics or enters the journal.

## Workspace identity and Git observation

Run initialization stores `workspace_identity` version 1 with the canonical absolute worktree root, canonical per-worktree Git directory, canonical Git common directory, and an ID framed from those paths as lowercase `sha256:<64 hex>`. Linked worktrees therefore have different IDs because their Git directories differ. The string in `next.workspace_identity` is this stable ID; it is never the root path. Every `base_argv` independently preserves the original canonical absolute value in exactly one `--root ROOT` option before any positional `--` separator.

Before Load, status-derived recovery, and every submit, answer, skip, stop, or resume mutation, Slipway rediscovers all three paths without a shell and compares the full identity. A reused root, another linked worktree, or moved/retargeted Git metadata fails before journal mutation with `workspace_identity_mismatch` and a read-only command `next` pointing at the Run's persisted worktree when that root is known. Repository-wide status listing is the deliberate read-only exception: a Run owned by another worktree is not fully replayed; only its valid `FirstEvent` header (`id`, `goal`, `workspace`, `workspace_identity`, initial `state`, and `created_at`) is returned with optional `workspace_foreign:true` and an inspect-in-owning-workspace `next`. An unreadable or invalid first event is still skipped.

`initial_git` and `current_git` are version 1 structured observations. Each contains `head`, an `index_fingerprint` over the exact bytes from `git ls-files --stage -z`, a `status_fingerprint` over the exact bytes from `git status --porcelain=v2 -z --untracked-files=all`, `path_count`, a `path_fingerprint` over every sorted dirty-path observation, bounded sorted non-null `dirty_files` and `path_observations` prefixes, an explicit `details_truncated` flag, and a `snapshot_hash` framed over every retained field plus the complete-set fingerprint. Porcelain-v2 ordinary, rename/copy (including the origin path), unmerged, and untracked records are parsed without losing spaces or Unicode. `initial_git` is immutable; routing compares the structured snapshot hashes.

Each retained path observation records path, category/state, known size, and a content fingerprint when readable. Regular dirty and untracked files up to 16 MiB receive a full streamed SHA-256 without retaining raw content. Larger files are explicitly classified `oversize` and receive a bounded, domain-separated fingerprint over size plus fixed first/middle/last samples; this detects size changes and changes in sampled regions but may miss an equal-length edit confined outside those regions. Symlinks are never followed and hash only their link target. Missing, non-regular, and unreadable states remain explicit and do not fail the whole Git observation. If detailed records exceed the bounded projection budget, the omitted count and complete `path_fingerprint` remain visible; no raw file content enters a journal.

## Action

Run mutation responses use the versioned state envelope described below. Whenever its `state` is `active`, the envelope contains a mandatory non-null `action` with this shape:

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

`kind` is `orient`, `clarify`, `implement`, `review`, or `summarize`. An ad-hoc Action omits both `source` and `requirements`; it never sends either field as `null`.

Every issue-bound Action includes both objects, but no chapter Markdown:

```json
{
  "source": {
    "kind": "change_issue",
    "canonical_url": "https://github.com/OWNER/REPOSITORY/issues/123",
    "issue_id": "I_...",
    "source_revision": "sha256:...",
    "manifest_revision": "sha256:...",
    "requirements_revision": "sha256:..."
  },
  "requirements": {
    "requirements_revision": "sha256:...",
    "sections": [
      {
        "key": "requirements",
        "role": "requirements",
        "title": "Requirements",
        "section_revision": "sha256:...",
        "material_sha256": "sha256:...",
        "bytes": 18231
      }
    ],
    "required_for_action": ["requirements"],
    "reader": {
      "operation": "read_material",
      "base_argv": [
        "slipway", "_machine", "material", "--root", "/absolute/workspace",
        "--run", "RUN", "--action", "ACTION"
      ],
      "input": {
        "name": "section",
        "type": "enum",
        "flag": "--section",
        "required": true,
        "choices": ["requirements"]
      }
    }
  }
}
```

The host resolves each key with the exact structured argv and receives one `action_material` JSON object. The operation reads only the Run-local digest blob, validates its digest, byte count, and section revision, and never accesses GitHub. Only the current non-void Action may read material; completed, replaced, stopped, and otherwise stale Actions are rejected so old work cannot continue executing.

Only a structurally confirmed Implement may carry `destructive_authorization`:

```json
{
  "destructive_authorization": {
    "request_id": "11111111-1111-4111-8111-111111111111",
    "originating_action_id": "...",
    "scope_version": 1,
    "scope_sha256": "sha256:...",
    "targets": [{"kind": "path", "value": "/absolute/target"}],
    "impact": "exact irreversible consequence",
    "confirmed_at": "2026-07-12T10:11:12Z"
  }
}
```

Target kinds are `path`, `git_ref`, `external_resource`, and `data_domain`. `request_id` is a canonical lowercase non-nil RFC UUID. Targets must already be unique and bytewise sorted by `(kind, value)`. Slipway recomputes SHA-256 over RFC 8785-compatible canonical JSON containing exactly `impact`, `request_id`, `scope_version: 1`, and `targets` in lexicographic key order.

Resource limits are:

- source file/raw envelope: 16 MiB;
- manifest JSON: 256 KiB;
- single normalized section: 256 KiB;
- complete bundle payload: 4 MiB;
- Outcome file/stdin: 1 MiB;
- single journal JSONL record: 4 MiB;
- Action `context`: 128 KiB;
- Action `brief`: 8 KiB;
- `suggested_actions`: at most 1 item;
- suggested Action `brief`: 8 KiB;
- total encoded Action: 256 KiB.

The `requirements` field is a bounded catalog; normative Markdown remains in separately addressable, untruncated local materials and is never copied into `context`. Context contains only active confirmed decisions and prior Outcome projections. Selection priority is the latest active decision, other active decisions newest-first, the most recent Outcome summary with its known issues, then remaining Outcomes newest-first. Superseded decisions and decisions bound to an older requirements revision are retained in history but excluded; a structured destructive confirmation attestation never contributes a product decision, while nonempty text from the separate decline-or-feedback branch may.

Each candidate normalizes CRLF and CR to LF and must be valid UTF-8. Selected items render chronologically inside the stable `Decisions:`, `Recent outcome:`, and `Earlier outcomes:` classes. If a normalized item does not fit, Slipway cuts only at a UTF-8 code-point boundary and appends exactly `...[truncated original_bytes=N sha256=HEX]`, where `HEX` hashes the complete normalized item. Omitted candidates produce a deterministic per-class `[omitted CLASS: N]` line. The result never exceeds exactly 128 KiB and replay of the same journal is byte-identical. Voided Action outcomes are excluded. Action size is measured with the same non-HTML-escaping JSON encoder used on stdout, so stored history cannot make the Action grow beyond the bounded context projection.

## Submit an Outcome

```bash
slipway _machine submit --run RUN --action ACTION --outcome-file outcome.json
slipway _machine submit --run RUN --action ACTION --outcome-stdin
```

Exactly one input mode is required. Slipway hashes the original accepted bytes before decoding. Retrying those exact bytes is idempotent; a semantically equivalent JSON object with different whitespace or key order conflicts.

Every public Outcome field is mandatory. Arrays must be arrays, including when empty. Inapplicable object branches must be JSON `null`:

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

`action_kind` is mandatory and must exactly equal the current Action's `kind`. Slipway rejects a missing, unknown, or mismatched value; there is no inference or legacy fallback.

Host status is only `completed`, `needs_input`, `partial`, or `error`. `skipped` is a CLI-owned `_machine skip` event and is rejected in a host Outcome. A skipped Review remains outcome-free and exposes the CLI-owned `review_projection.result: "not_run"` in action history, so consumers never confuse it with a host review result. Outcome input is capped at 1 MiB and must be UTF-8 without a BOM or trailing data.

An Orient or Clarify may suggest at most one immediate `clarify`, `implement`, or `summarize` Action:

```json
{"kind": "clarify", "brief": "one bounded next decision"}
```

Implement, Review, Summary, and every `needs_input` Outcome use an empty `suggested_actions` array.

### Pause

A `needs_input` Outcome has a non-null `pause`; every other status has `pause: null`:

```json
{
  "reason": "decision_required",
  "question": "one human decision",
  "destructive_request": null,
  "supersedes_answer_action_id": "previous-answer-action-id"
}
```

Host pause reasons are `decision_required`, `destructive_confirmation_required`, and `environment_unavailable`. `budget_exhausted` is CLI-owned and is rejected from a host. A decision pause may optionally include `supersedes_answer_action_id`; it is invalid on environment or destructive pauses. Its value is the `action_id` of a previously recorded answer that is still active, is not a destructive authorization, and belongs to the current requirements revision (or the current ad-hoc requirements context). The field records explicit supersession intent: only when the user answers this waiting decision does the named answer become inactive while remaining in history, and the new answer becomes active. Skipping leaves the prior answer active; prose never implies supersession, and unrelated answers remain active. A destructive request is required only for an Implement destructive pause:

```json
{
  "reason": "destructive_confirmation_required",
  "question": "Confirm this exact destructive scope?",
  "destructive_request": {
    "request_id": "11111111-1111-4111-8111-111111111111",
    "targets": [{"kind": "path", "value": "/absolute/target"}],
    "impact": "exact irreversible consequence",
    "scope_sha256": "sha256:..."
  }
}
```

```text
slipway _machine answer --run RUN --action ACTION --root ROOT --text TEXT
slipway _machine answer --run RUN --action ACTION --root ROOT --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
```

Normal decision answers require text and forbid destructive flags; an explicitly named prior answer is superseded only when the revising question is answered. Environment pauses reject answers and must resume. All three host pauses retain the no-reason `skip-action` control. `--confirm-destructive` is a trusted-host attestation of a current user confirmation, not cryptographic proof of human presence; a malicious process with shell authority can forge flags. Natural-language answer text, including `yes`, never grants destructive authority. It records feedback or decline, invalidates the waiting Action and queues, clears the request/grant, and produces a fresh non-destructive Orient. Confirmation requires `--confirm-destructive --scope-sha256 DIGEST`; the digest must exactly match the CLI-recomputed current request, whose `request_id` is a canonical lowercase non-nil RFC UUID. Success records an attestation and issues exactly one fresh Implement carrying a field-for-field copy as `destructive_authorization`. Any changed or expanded target/impact requires a new request.

### Implementation

A non-paused Implement has a non-null `implementation` and `review: null`:

```json
{
  "result": "applied",
  "files_changed": [],
  "activities": [
    {"kind": "test", "command": "go test ./...", "exit_code": 0, "summary": "passed"}
  ],
  "uncertainties": [],
  "attempts": 1
}
```

The exact matrix is:

| Host status | `implementation.result` |
| --- | --- |
| `completed` | `applied` or `not_needed` |
| `partial` | `partial` |
| `error` | `unable` |
| `needs_input` | `implementation: null` plus a valid pause |

Activity kind is `test`, `typecheck`, `build`, or `lint`. Zero activities are legal. A final report with none includes exactly:

```text
No test, typecheck, build, or lint activity was reported.
```

### Review

A non-paused Review has a non-null `review` and `implementation: null`:

```json
{
  "result": "findings_reported",
  "findings": [
    {"location": "path:line or surface", "summary": "finding", "detail": "supporting detail"}
  ],
  "uncertainties": []
}
```

The exact matrix is:

| Host status | `review.result` |
| --- | --- |
| `completed` | `no_findings_reported` or `findings_reported` |
| `partial` | `inconclusive` |
| `error` | `error` |

`findings_reported` requires at least one finding; `no_findings_reported` requires none. Review never uses `needs_input` and never suggests or schedules repair. `not_run` is only a CLI review-skip projection and is rejected from a host Outcome.

### Other Action kinds

- Orient supports `completed`, `partial`, `error`, and non-destructive `needs_input`.
- Clarify supports `completed`, `error`, and non-destructive `needs_input`; `partial` is invalid.
- Summary supports only `completed` and `error`, with no suggestions or result branches.

## Deterministic routing

```text
needs_input                                         → paused by pause.reason
orient/clarify/implement + newly observed revision
  + review on                                      → review; discard pending suggestion
  pending valid suggestion with no review override   → suggested Action
orient/clarify/implement without either route      → summarize
any legal review result                            → summarize
summary result                                     → ended
next Action with no budget                         → paused (budget_exhausted)
```

Implementation reports, activity exit codes, and Review findings are data, not routing gates. Orient, Clarify, and Implement each compare the new CLI Git observation with the previously observed snapshot. A newly observed revision routes to Review before any host suggestion when Review is enabled, so an Orient/Clarify suggestion cannot bypass inspection; that pending suggestion is discarded and Review follows its normal Summary route. A later revision after a completed or skipped Review therefore receives a new Review, while unchanged snapshots do not loop. This applies even when an Implement host reported `not_needed`, `partial`, or `unable`. Review findings go to Summary and do not create an automatic repair loop.

Skip is diff-first: skipping Orient, Clarify, or Implement observes current Git and routes to Review whenever that observation differs from the previously observed snapshot and review is enabled; otherwise it routes to Summary. Review skip routes to Summary. Summary skip writes a minimal CLI-owned factual summary and ends. Every skip clears destructive request/grant state.

Every observed start-to-current difference records the factual `observed_since_start` observation and an `attribution_uncertainty`: concurrent user edits, another Run, or tools may have contributed. Slipway never assigns the difference to a host or Run. Both discrepancy directions are retained neutrally as report observations: `applied|partial` with no observed difference, and `not_needed|unable` with an observed difference. Routing remains diff-first. Review briefs and final Summary preserve this uncertainty and the structured observations for paths already dirty at Run start.

## Idempotency and ordering

An `action_id` accepts one Outcome. Retrying the exact original Outcome bytes is idempotent even after the Run becomes stopped or ended; a different byte payload conflicts even when it decodes to the same JSON values. A voided Action always rejects, and a non-recorded late submission to a non-current, stopped, or ended Action rejects. Answer idempotency hashes the canonical tuple `(action_id,text,confirm_destructive,scope_sha256)`: an identical retry returns the current result without an event, budget use, or Action, while another payload conflicts. Every successful resume voids an outstanding Action, destructive request/grant, and queue before fresh Orient; candidate creation instead voids them and pauses without an Action.

## Journal commit errors

`.git/slipway/runs/<run-id>/journal.jsonl` is the sole recovery authority; `run.json` is only a replaceable projection, and `run.lock` is a coordination artifact rather than Run authority. Immutable initialization inspection never creates the lock, so directories with an absent/corrupt initialization record or a foreign workspace remain untouched. After that record identifies a valid local Run, locked replay or mutation may recreate a missing lock before continuing. Machine errors for storage mutations include stable `details.phase`, `details.committed`, `details.projection_stale`, `details.namespace_detached`, and `details.ambiguous` fields. `mutation_committed_projection_stale` means the journal event was fsynced but projection completion failed. `mutation_outcome_ambiguous` means an inode was written but durability or namespace membership could not be proved. Both return `next.operation:"none"`: inspect/replay the journal before recovery and never blind-retry. A pre-write failure uses `mutation_not_committed`. If a single journal record would exceed the 4 MiB cap (for example a legitimate large Outcome accumulation that overflows a Summary record), the Submit returns `journal_record_too_large` with a recoverable read-only `inspect-run` command (e.g. `slipway status --root <workspace>`); the persistent Run is not killed and can still be inspected or skipped, so this error never returns `next.operation:"none"`.

The storage capability is `file_and_directory_fsync` on supported Unix-like systems. On Windows it is stably reported as `file_fsync_only` with `directory_sync:false` and limitation `directory_fsync_unsupported`; file contents are fsynced, but crash durability of newly created or renamed directory entries cannot be claimed.

Run mutation command responses use one envelope containing `contract_version`, `run_id`, `state`, and structured `next`. Active responses always contain a non-null `action`; other states contain it when a current Action remains and omit it otherwise. Applicable optional fields are `pause_reason`, `summary`, `pinned_source`, `source_candidate`, `resume_operation`, and `budget_applied`. A single `status RUN --json` is the stable flat Run projection with mandatory top-level `contract_version`, `review_enabled`, durable `review_pending`, and freshly derived `next`; `review_pending` survives decision, environment, budget, stop/resume, and source-choice interruptions until the corresponding Review is completed or explicitly skipped. `status --json` is exactly `{contract_version,runs:[...]}`, including `{"contract_version":2,"runs":[]}` when empty. Rendered commands are never journaled. Errors contain `contract_version`, `code`, `message`, structured `next`, `exit_code`, and optional strictly shaped `details`. Run states are `active`, `paused`, `stopped`, and `ended`.

## Public report envelopes and doctor advisories

Every JSON success/error is an unambiguous top-level contract-version-2 object. Install and uninstall use exactly `{contract_version,hosts,transaction_outcome,written,removed,preserved,recovery_artifacts,warnings}` with all arrays present. List uses `{contract_version,hosts:[{id,detected,installed,needs_refresh,capabilities,warning?}]}`. Each host entry carries an optional `warning` string when a read-only listing could not fully inspect that host (for example an unreadable or malformed manifest); because listing is non-mutating, one host's inspection problem degrades to this advisory instead of aborting the whole report. Doctor uses `{contract_version,checks:[...]}`; every check has `{code,status,host_id,name,detail}`, and the `runstore_durability_full|runstore_durability_limited` check additionally has `durability:{level,file_sync,directory_sync,limitation?}`. Check status is only `ok|warning|error`. The normative schema closes every object with `additionalProperties:false`. Repository/adapter codes are `repository_ok`, `adapter_manifest_unreadable`, `adapter_not_detected`, `adapter_not_installed`, `adapter_refresh_required`, `adapter_modified`, and `adapter_healthy`.

Run/source size and history error codes are:

- `action_too_large` — the encoded Action exceeds 256 KiB after bounded context projection, so Slipway refuses to issue or persist it;
- `source_history_fork` — a refreshed manifest declares a parent requirements revision other than the pinned revision, so refresh is rejected before Run mutation and the source must start a new Run;
- `source_history_in_place_edit` — a previously accepted comment identity was rebound or its accepted section changed in place instead of being replaced;
- `source_integrity_mismatch` — the requirements revision changed while the manifest revision did not;
- `source_alias_limit` — same-Issue transfer history would exceed 64 retained URL aliases, so recovery starts a new Run from the refreshed source;
- `journal_record_too_large` - one JSONL record would exceed 4 MiB; nothing is appended, and recovery provides a read-only `inspect-run` command while the existing Run remains inspectable and skippable.
Adapter `transaction_outcome` is one of `committed`, `rolled_back`, `not_committed`, and `ambiguous`. Only `committed` retains planned `written`/`removed` claims. `preserved` contains ordinary ownership-safe user content; actual retained recovery/quarantine paths are separately listed in `recovery_artifacts`. A committed cleanup error may therefore report both committed changes and recovery artifacts, while an ambiguous rollback reports no planned changes.

GitHub capability codes are `github_cli_unavailable`, `github_cli_version_unknown`, `github_cli_rest_fallback_required`, `github_cli_compatible`, `github_auth_unavailable`, `github_auth_available`, `github_issue_permissions_ok`, `github_issue_permissions_limited`, and `github_issue_permissions_unknown`. Version detection and `gh auth status --hostname github.com` are time-bounded; `gh <2.94.0` reports that the official REST fallback is required for parent/sub-issue/dependency operations. Permission lookup runs only for a safely identified credential-free GitHub origin and never reports raw command, token, authentication, or API output.

Legacy namespace advisories use `legacy_runtime_residue`, `legacy_cache_residue`, `legacy_scope_root_residue`, `legacy_scopes_residue`, `legacy_locks_residue`, `legacy_processes_residue`, `legacy_repair_backups_residue`, or `legacy_unknown_residue`. Doctor discovers the Git common directory without opening runstore, lists only exact top-level names by metadata, excludes current `runs`, and never reads, migrates, or deletes residue. Guidance is to stop the old binary, back up, and clean manually if desired. GitHub and legacy warnings alone do not block doctor and have no effect on ad-hoc Run health.
