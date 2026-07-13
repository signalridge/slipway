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

The first form is ad-hoc only and rejects every source option. An issue-bound Run with no current candidate requires exactly one of a freshly imported envelope or `--use-pinned-source`; omission never means “unchanged.” A Run with a current candidate rejects refresh/pinned modes and requires an exact `--source-choice` plus `--candidate` pair. An invalid candidate permits only `pinned`.

Refresh validates provider/host and issue node ID before any mutation. A different issue is rejected and requires a new Run. Repository, number, or URL transfer updates projection and records the prior canonical URL once while still comparing the marker and requirements. A refresh whose manifest revision is unchanged—including identical, projection-only, and other non-material drift—voids the outstanding Action/queue/authorization and issues a fresh Orient. Any new manifest revision, including a content-identical replacement, or an invalid body stores a run-local, path-free candidate, voids outstanding work, and pauses with `decision_required` without applying the requested budget.

`pinned` keeps the accepted snapshot; `adopt` installs a valid candidate. Only when adoption changes `requirements_revision` are answers derived from the old revision removed from active Action context; their records remain. A content-identical manifest-only replacement keeps those answers active. The choice receipt makes an identical `(candidate_id, choice)` retry a no-op; another choice or stale ID conflicts. `--use-pinned-source` records `source_refresh_skipped` and never claims the source was unchanged.

An explicit resume budget must be 1..1000 and replaces the remainder before fresh Orient consumes one. If omitted, a positive remainder is preserved; an exhausted remainder becomes `max(initial_budget, 3)` before Orient. Candidate creation reports `budget_applied: false`; repeat the budget on the subsequent choice. Paused protocol output exposes safe `pinned_source`, `source_candidate`, `resume_operation`, and `budget_applied` fields. Full status JSON also exposes `last_resume_result` and the last choice receipt, but no source-file path.

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

Input types are `string`, `path`, `enum`, and `digest`; enum inputs list nonempty `choices`, and digest values use lowercase `sha256:<64 hex>`. Resolve a chosen variant by copying `base_argv`, then, in input schema order, append each supplied `flag` and exact raw value as separate argv elements. Unknown, missing required, wrong-type, invalid enum, and malformed digest values are rejected. Every variant contains the run's original absolute `--root`; no variant contains `FILE`, `<file>`, `<answer>`, or a quoted pseudo-value.

Active Actions derive `submit-outcome-file`, inputless `submit-outcome-stdin`, and inputless `skip-action`. Decision pauses derive `answer-decision`. Destructive pauses derive inputless `confirm-destructive` fixed to the current digest (with optional text) plus `decline-or-feedback` requiring text. Ad-hoc recovery derives `resume-ad-hoc`; issue recovery derives `refresh-source` and `use-pinned-source`; valid candidates derive `keep-pinned` and `adopt`, while invalid candidates expose only `keep-pinned`. Ended Runs use operation `none` and an empty variants array.

Only a variant with no unresolved required input may be rendered as a display command. Rendering happens at the CLI edge from argv for POSIX, `cmd.exe`, or PowerShell and never changes machine semantics or enters the journal.

## Workspace identity and Git observation

Run initialization stores `workspace_identity` version 1 with the canonical absolute worktree root, canonical per-worktree Git directory, canonical Git common directory, and an ID framed from those paths as lowercase `sha256:<64 hex>`. Linked worktrees therefore have different IDs because their Git directories differ. The string in `next.workspace_identity` is this stable ID; it is never the root path. Every `base_argv` independently preserves the original canonical absolute value after exactly one `--root`.

Before Load, status-derived recovery, and every submit, answer, skip, stop, or resume mutation, Slipway rediscovers all three paths without a shell and compares the full identity. A reused root, another linked worktree, or moved/retargeted Git metadata fails before journal mutation with `workspace_identity_mismatch`, `next.operation:"none"`, and no retry variant.

`initial_git` and `current_git` are version 1 structured observations. Each contains `head`, an `index_fingerprint` over the exact bytes from `git ls-files --stage -z`, a `status_fingerprint` over the exact bytes from `git status --porcelain=v2 -z --untracked-files=all`, sorted non-null `dirty_files`, sorted non-null `path_observations`, and a `snapshot_hash` framed over every structured field. Porcelain-v2 ordinary, rename/copy (including the origin path), unmerged, and untracked records are parsed without losing spaces or Unicode. `initial_git` is immutable; routing compares the structured snapshot hashes.

Each path observation records path, category/state, known size, and a content digest when safe. Regular dirty and untracked files up to 16 MiB are hashed; symlinks are never followed and hash only their link target. Missing, symlink, non-regular, unreadable, and oversize states are explicit. Oversize files are rejected from content hashing by size before read, and unreadable/oversize paths do not fail the whole Git observation. No raw file content enters a journal. A same-size content change wholly inside an oversize file is outside this bounded observer and may require host inspection.

## Action

Active responses are bare Action JSON objects:

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
    "request_id": "...",
    "originating_action_id": "...",
    "scope_version": 1,
    "scope_sha256": "sha256:...",
    "targets": [{"kind": "path", "value": "/absolute/target"}],
    "impact": "exact irreversible consequence",
    "confirmed_at": "2026-07-12T10:11:12Z"
  }
}
```

Target kinds are `path`, `git_ref`, `external_resource`, and `data_domain`. Targets must already be unique and bytewise sorted by `(kind, value)`. Slipway recomputes SHA-256 over RFC 8785-compatible canonical JSON containing exactly `impact`, `request_id`, `scope_version: 1`, and `targets` in lexicographic key order.

Action limits are:

- `context`: 128 KiB;
- `brief`: 8 KiB;
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

Host status is only `completed`, `needs_input`, `partial`, or `error`. `skipped` is a CLI-owned `_machine skip` event and is rejected in a host Outcome. Outcome input is capped at 1 MiB and must be UTF-8 without a BOM or trailing data.

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
  "destructive_request": null
}
```

Host pause reasons are `decision_required`, `destructive_confirmation_required`, and `environment_unavailable`. `budget_exhausted` is CLI-owned and is rejected from a host. A destructive request is required only for an Implement destructive pause:

```json
{
  "reason": "destructive_confirmation_required",
  "question": "Confirm this exact destructive scope?",
  "destructive_request": {
    "request_id": "...",
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

Normal decision answers require text and forbid destructive flags; environment pauses reject answers and must resume. `--confirm-destructive` is a trusted-host attestation of a current user confirmation, not cryptographic proof of human presence; a malicious process with shell authority can forge flags. Natural-language answer text, including `yes`, never grants destructive authority. It records feedback or decline, invalidates the waiting Action and queues, clears the request/grant, and produces a fresh non-destructive Orient. Confirmation requires `--confirm-destructive --scope-sha256 DIGEST`; the digest must exactly match the CLI-recomputed current request. Success records an attestation and issues exactly one fresh Implement carrying a field-for-field copy as `destructive_authorization`. Any changed or expanded target/impact requires a new request.

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

`.git/slipway/runs/<run-id>/journal.jsonl` is the sole recovery authority; `run.json` is only a replaceable projection. Machine errors for storage mutations include stable `details.phase`, `details.committed`, `details.projection_stale`, `details.namespace_detached`, and `details.ambiguous` fields. `mutation_committed_projection_stale` means the journal event was fsynced but projection completion failed. `mutation_outcome_ambiguous` means an inode was written but durability or namespace membership could not be proved. Both return `next.operation:"none"`: inspect/replay the journal before recovery and never blind-retry. A pre-write failure uses `mutation_not_committed`.

The storage capability is `file_and_directory_fsync` on supported Unix-like systems. On Windows it is stably reported as `file_fsync_only` with `directory_sync:false` and limitation `directory_fsync_unsupported`; file contents are fsynced, but crash durability of newly created or renamed directory entries cannot be claimed.

Paused, stopped, and ended command responses contain `contract_version`, `run_id`, `state`, structured `next`, and applicable `pause_reason`, `summary`, `pinned_source`, `source_candidate`, `resume_operation`, and `budget_applied` fields. A single `status RUN --json` is the stable flat Run projection with mandatory top-level `contract_version`, `review_enabled`, durable `review_pending`, and freshly derived `next`; `review_pending` survives decision, environment, budget, stop/resume, and source-choice interruptions until the corresponding Review is completed or explicitly skipped. `status --json` is exactly `{contract_version,runs:[...]}`, including `{"contract_version":2,"runs":[]}` when empty. Rendered commands are never journaled. Errors contain `contract_version`, `code`, `message`, structured `next`, `exit_code`, and optional strictly shaped `details`. Run states are `active`, `paused`, `stopped`, and `ended`.

## Public report envelopes and doctor advisories

Every JSON success/error is an unambiguous top-level contract-version-2 object. Install and uninstall use exactly `{contract_version,hosts,written,removed,preserved,warnings}` with all arrays present. List uses `{contract_version,hosts:[{id,detected,installed,needs_refresh,capabilities}]}`. Doctor uses `{contract_version,checks:[{code,status,host_id,name,detail}]}`; check status is only `ok|warning|error`. The normative schema closes every object with `additionalProperties:false`. Repository/adapter codes are `repository_ok`, `adapter_manifest_unreadable`, `adapter_not_detected`, `adapter_not_installed`, `adapter_refresh_required`, `adapter_modified`, and `adapter_healthy`.

GitHub capability codes are `github_cli_unavailable`, `github_cli_version_unknown`, `github_cli_rest_fallback_required`, `github_cli_compatible`, `github_auth_unavailable`, `github_auth_available`, `github_issue_permissions_ok`, `github_issue_permissions_limited`, and `github_issue_permissions_unknown`. Version detection and `gh auth status --hostname github.com` are time-bounded; `gh <2.94.0` reports that the official REST fallback is required for parent/sub-issue/dependency operations. Permission lookup runs only for a safely identified credential-free GitHub origin and never reports raw command, token, authentication, or API output.

Legacy namespace advisories use `legacy_runtime_residue`, `legacy_cache_residue`, `legacy_scope_root_residue`, `legacy_scopes_residue`, `legacy_locks_residue`, `legacy_processes_residue`, `legacy_repair_backups_residue`, or `legacy_unknown_residue`. Doctor discovers the Git common directory without opening runstore, lists only exact top-level names by metadata, excludes current `runs`, and never reads, migrates, or deletes residue. Guidance is to stop the old binary, back up, and clean manually if desired. GitHub and legacy warnings alone do not block doctor and have no effect on ad-hoc Run health.
