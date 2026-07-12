# Architecture

> English is non-normative; see the [Chinese product contract](../zh/reference/product-contract.md).

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil (only required low-level primitives)
```

- `cmd` owns exactly seven public commands, hidden versioned machine commands, and text/JSON rendering.
- `autopilot` owns strict Action/Outcome unions, source envelopes/revisions/candidates, budget, routing, destructive authorization, and structured `next` values.
- `runstore` discovers canonical Git identity and maintains anchored append-only journals plus replaceable projections.
- `adapter` plans ownership-safe generation for ten hosts; `tmpl` embeds exactly six explicit capabilities and the attributed `grill-me` reference.
- `fsutil` supplies rooted atomic transactions, Git discovery, symlink/reparse defenses, and rollback post-state validation.
- `recoverycmd` consumes complete argv only and renders POSIX/cmd/PowerShell display commands. It does not read journals or decide routes; autopilot does not depend on it.

At Run start the CLI stores immutable workspace identity and a Git fingerprint: exact index and porcelain-v2 bytes plus sorted metadata/digests for pre-existing dirty/untracked paths. Recovery revalidates identity before load and mutation. Observed-since-start difference drives safety-side Review routing but never proves the Run caused a change.

GitHub publication remains host-side. The Go binary holds no provider token: it strictly validates a host-attested raw Change envelope and journals a normalized pinned snapshot. Publication capabilities use approved operation/item markers and reconciliation, not a repository runtime.

There is no model provider, old-state reader, compatibility alias, dual runtime, ambient activation, required-command registry, Spec/artifact lifecycle, worktree binding, or automatic review-repair loop. Historical data remains untouched and ignored.
