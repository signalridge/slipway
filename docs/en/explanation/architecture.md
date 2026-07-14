# Architecture

> English is non-normative; see the [Chinese product contract](../../zh/reference/product-contract.md).

Slipway is a small control plane for bounded, recoverable AI-assisted work. It does not replace an AI coding tool, project tracker, or Git. It schedules one versioned Action at a time, observes Git independently, pins the source by digest, and stores recovery history — all while the host executes the work.

## Dependency direction

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil (only required low-level primitives)
```

Dependency direction is fixed and enforced by an architecture guard test. `autopilot` only emits structured `next` values and never depends on `recoverycmd`; `recoverycmd` never reads a journal or decides a route.

## Packages

| Package | Responsibility |
| --- | --- |
| `cmd` | The seven public commands, hidden versioned machine commands, and text/JSON rendering. |
| `internal/autopilot` | Strict Action/Outcome unions, source envelopes/revisions/candidates, budget, routing, destructive authorization, and structured `next` values. |
| `internal/runstore` | Discovers canonical Git identity and maintains anchored append-only journals plus replaceable projections. |
| `internal/adapter` | Plans ownership-safe capability generation for ten hosts. |
| `internal/tmpl` | Embeds exactly six explicit capabilities and the attributed `grill-me` reference. |
| `internal/fsutil` | Rooted atomic transactions, Git discovery, symlink/reparse defenses, and rollback post-state validation. |
| `internal/recoverycmd` | Consumes complete argv only and renders POSIX/cmd/PowerShell display commands. Never reads journals or decides routes. |
| `internal/jsonstrict` | Shared structural scanner for duplicate-key, valid-JSON, and trailing-data rejection. |
| `internal/testlint` | Repository test policy analyzer. |

## Run start and Git observation

At Run start the CLI stores an immutable workspace identity and a Git fingerprint: the exact index and porcelain-v2 bytes plus sorted metadata/digests for every pre-existing dirty/untracked path. Recovery revalidates identity before load and mutation. A reused root, another linked worktree, or moved/retargeted Git metadata fails with `workspace_identity_mismatch` before any journal mutation.

Observed-since-start difference drives safety-side Review routing but never proves the Run caused a change. Concurrent user edits, another Run, or other tools may all contribute. Slipway records the factual `observed_since_start` observation and an `attribution_uncertainty`, and never assigns the difference to a host or Run.

## Hosts and GitHub

The Go binary holds no provider token. It strictly validates a host-attested raw Change envelope and journals a normalized pinned snapshot. GitHub reading and writing happen on the host side with the user's own authenticated tools; publication uses approved operation/item UUID markers and reconciliation — not a repository Change runtime.

There is no model provider, old-state reader, compatibility alias, dual runtime, ambient activation, required-command registry, Spec/artifact lifecycle, worktree binding, or automatic review-repair loop. Historical data and legacy namespaces remain untouched and ignored.

## Not introduced

```text
internal/change   internal/spec   internal/plan
internal/lifecycle   internal/gate   internal/tracker runtime
```

See [Product authority](../reference/product-overview.md), [Machine protocol](../reference/machine-protocol.md), and the [decision to use manifest-addressed source bundles](../../decisions/0001-source-bundle-v2.md).
