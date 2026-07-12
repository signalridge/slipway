# Runs and privacy

Run recovery data lives in the Git common directory so linked worktrees see the same run list while each journal records its owning canonical workspace.

```text
.git/slipway/runs/<run-id>/
  journal.jsonl
  run.json
  run.lock
```

Directories are created with current-user-only intent (`0700` on Unix) and leaf files with `0600`. `journal.jsonl` is the sole append-only recovery authority; `run.json` is a replaceable projection and `run.lock` serializes journal mutation only. A legacy `events.jsonl` or `runtime/cache/scope-root/scopes/locks/processes/repair-backups` sibling is unowned residue: Slipway ignores it, doctor inspects only top-level names for advisory reporting, and no command reads, migrates, aliases, or deletes its task content.

## Stored data and privacy promise

A journal contains the original goal, canonical workspace identity, immutable initial Git observation, structured Git deltas, accepted five-section Requirements for an issue-bound source, Actions, Outcomes, user answers and supersession metadata, skips, stops, source choices, destructive requests/grants, budgets, reported activity command summaries, known issues, and uncertainties. **Goals, accepted Requirements, answers, and truthful command summaries may contain sensitive text.** Treat `.git/slipway/runs/` as local private data and warn before source import or journal creation.

Slipway does not promise that a journal contains no secrets. It does promise data minimization: it does not intentionally collect a GitHub token, credential store, raw Issue body, raw/full comments, environment-variable dump, unrelated file contents, full conversation transcript, or hidden reasoning. Source import persists accepted sections, identity, and revisions rather than the raw body. Git path observations contain category/state, size, and bounded SHA-256 metadata rather than raw file content; regular hashing stops at 16 MiB and records oversize/unreadable states.

Generated hosts must redact recognized credential values before publication or journaling while preserving truthful command identity—for example the executable and the position/name of a redacted argument. Recognition is fallible, so users must not paste secrets. A public repository Issue has no private switch; sensitive work belongs in a private repository, private vulnerability reporting only for a real vulnerability when enabled, an existing security channel, or an ad-hoc Run.

Action context is bounded at 128 KiB and is not a full replay. Requirements remain separate; context deterministically selects active decisions and Outcome summaries/known issues, normalizes newlines, truncates on UTF-8 boundaries with byte-count/SHA-256 markers, and records omission counts.

## Permissions, retention, and removal

Unix modes protect against ordinary other-user access, not root, backups, malware, or another process under the same UID. Windows uses current-user ACL intent, but inherited ACLs, administrators, backup agents, and same-account processes may still read data; there is no absolute Windows ACL guarantee. Repository owners should define retention, inspect ACLs, protect backups, and avoid publishing `.git/slipway/runs/`.

Deleting a run directory removes Slipway's recovery capability and journal projection only. It does not modify Git, source files, Issue/deployment state, replicas, snapshots, cloud backups, filesystem remnants, or encryption keys. It is not secure erase, backup purge, or key destruction.

## Commit and recovery semantics

A mutation is committed only after journal bytes and the journal file are fsynced. Projection data is encoded/fsynced to a temporary file, renamed to `run.json`, then followed by directory sync where supported. If a later projection step fails, CLI reports `mutation_committed_projection_stale` with committed/projection-stale details and no retry operation. Replay the authoritative journal; do not blindly retry.

Before Load/status recovery and mutation, Slipway rediscovers workspace identity. A mismatch returns `workspace_identity_mismatch` without modifying the journal. Interrupted final records are repaired on the same verified handle; earlier corruption is rejected. Windows flushes journal/lock/projection files but lacks equivalent directory fsync, reported as `file_fsync_only` with `directory_fsync_unsupported`; it cannot claim Unix-equivalent crash durability for new or renamed directory entries.
