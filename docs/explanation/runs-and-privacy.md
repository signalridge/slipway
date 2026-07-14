# Runs and privacy

Run recovery data lives in the Git common directory, but workspace identity remains per worktree. A no-ID `status` lists only runs owned by the current canonical workspace; runs from sibling linked worktrees remain physically discoverable by exact ID but are rejected for status-derived recovery or mutation outside their original workspace. Each journal records that owning canonical workspace.

```text
.git/slipway/runs/<run-id>/
  journal.jsonl
  run.json
  run.lock
  materials/
    <64-lowercase-hex>
```

Directories are created with current-user-only intent (`0700` on Unix) and leaf files with `0600`. `journal.jsonl` is the sole append-only state-transition authority; `run.json` is a replaceable projection, `run.lock` serializes journal mutation, and immutable `materials/` blobs provide the chapter bytes referenced by journaled catalogs. A legacy `events.jsonl` or `runtime/cache/scope-root/scopes/locks/processes/repair-backups` sibling is unowned residue: Slipway ignores it, doctor inspects only top-level names for advisory reporting, and no command reads, migrates, aliases, or deletes its task content.

## Stored data and privacy promise

A journal contains the original goal, canonical workspace identity, immutable initial Git observation, structured Git deltas, a bounded issue-source chapter catalog and provenance, Actions, Outcomes, user answers and supersession metadata, skips, stops, source choices, destructive requests/grants, budgets, reported activity command summaries, known issues, and uncertainties. Accepted chapter Markdown is stored once in private content-addressed `materials/` files, not duplicated into every Action or journal event. **Goals, accepted Requirements, answers, and truthful command summaries may contain sensitive text.** Treat `.git/slipway/runs/` as local private data and warn before source import or journal creation.

Slipway does not promise that a journal contains no secrets. It does promise data minimization: it does not intentionally collect a GitHub token, credential store, raw Issue body, unreferenced discussion comments, environment-variable dump, unrelated file contents, full conversation transcript, or hidden reasoning. Source import persists only manifest-referenced normalized chapter payloads plus their bounded catalog/provenance; comment markers and the raw enclosing comment bodies are not retained. Git path observations contain category/state, size, and bounded SHA-256 metadata rather than raw file content; regular hashing stops at 16 MiB and records oversize/unreadable states.

Generated hosts must redact recognized credential values before publication or journaling while preserving truthful command identity—for example the executable and the position/name of a redacted argument. Recognition is fallible, so users must not paste secrets. A public repository Issue has no private switch; sensitive work belongs in a private repository, private vulnerability reporting only for a real vulnerability when enabled, an existing security channel, or an ad-hoc Run.

Action context is bounded at 128 KiB and is not a full replay. Requirements remain separate as an ordered catalog and local `_machine material` reader; context deterministically selects active decisions and non-void Outcome summaries/known issues, normalizes newlines, truncates on UTF-8 boundaries with byte-count/SHA-256 markers, and records omission counts.

## Permissions, retention, and removal

Unix modes protect against ordinary other-user access, not root, backups, malware, or another process under the same UID. Windows uses current-user ACL intent, but inherited ACLs, administrators, backup agents, and same-account processes may still read data; there is no absolute Windows ACL guarantee. Repository owners should define retention, inspect ACLs, protect backups, and avoid publishing `.git/slipway/runs/`.

Namespace mutation has a separate, narrower guarantee. Anchored handles and long-lived identity pins defend parent traversal, identity reuse, and replacements observed at validation checkpoints. Portable POSIX `unlinkat` removes a name relative to a parent descriptor; it does not compare the final directory entry with an already-open leaf handle. Slipway therefore does not claim linearizable exact-object deletion against a continuously racing same-UID watcher in the final validation-to-unlink gap on platforms without an exact native primitive. Private randomized quarantine, atomic no-replace relocation, revalidation, and post-checks minimize the gap and preserve every replacement observed at a checkpoint. Root, malware, and same-account racing through that final syscall gap remain explicit residual limitations.

Deleting a run directory removes Slipway's recovery capability and journal projection only. It does not modify Git, source files, Issue/deployment state, replicas, snapshots, cloud backups, filesystem remnants, or encryption keys. It is not secure erase, backup purge, or key destruction.

## Commit and recovery semantics

Before a journal event may reference new source material, every referenced material blob is written to a temporary `0600` file, fsynced, renamed by digest, verified, and followed by directory sync where supported. A journal mutation is committed only after journal bytes and the journal file are fsynced. Projection data is encoded/fsynced to a temporary file, renamed to `run.json`, then followed by directory sync where supported. If a later projection step fails, CLI reports `mutation_committed_projection_stale` with committed/projection-stale details and no retry operation. Replay the authoritative journal; do not blindly retry.

Before Load/status recovery and mutation, Slipway rediscovers workspace identity. A mismatch returns `workspace_identity_mismatch` without modifying the journal. Interrupted final records are repaired on the same verified handle; earlier corruption is rejected. Windows flushes journal/lock/projection files but lacks equivalent directory fsync, reported as `file_fsync_only` with `directory_fsync_unsupported`; it cannot claim Unix-equivalent crash durability for new or renamed directory entries.
